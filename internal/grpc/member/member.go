package member

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	memberpb "github.com/werbot/werbot/internal/grpc/member/proto"
	"github.com/werbot/werbot/internal/grpc/project"
	userpb "github.com/werbot/werbot/internal/grpc/user/proto"
	"github.com/werbot/werbot/internal/trace"
)

// ListProjectMembers is ...
func (h *Handler) ListProjectMembers(ctx context.Context, in *memberpb.ListProjectMembers_Request) (*memberpb.ListProjectMembers_Response, error) {
	response := new(memberpb.ListProjectMembers_Response)

	sqlSearch := h.DB.SQLAddWhere(in.GetQuery())
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := h.DB.Conn.QueryContext(ctx, `SELECT
			"project_member"."id" AS "member_id",
			"project"."owner_id" AS "owner_id",
			"owner"."name" AS "owner_name",
			"project_member"."project_id" AS "project_id",
			"project"."title" AS "project_name",
			"project_member"."user_id" AS "user_id",
			"member"."name" AS "user_name",
			"project_member"."role" AS "member_role",
			"project_member"."active" AS "member_active",
			"project_member"."online" AS "member_online",
			"project_member"."created" AS "member_created",
			( SELECT COUNT (*) FROM "server_member" WHERE "member_id" = "project_member"."id"  ) AS "count_servers"
		FROM "project_member"
			INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
			INNER JOIN "user" AS "member" ON "project_member"."user_id" = "member"."id"
			INNER JOIN "user" AS "owner" ON "project"."owner_id" = "owner"."id"`+sqlSearch+sqlFooter)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	for rows.Next() {
		var role string
		var created pgtype.Timestamp
		member := new(memberpb.ProjectMember_Response)
		err = rows.Scan(
			&member.MemberId,
			&member.OwnerId,
			&member.OwnerName,
			&member.ProjectId,
			&member.ProjectName,
			&member.UserId,
			&member.UserName,
			&role,
			&member.Active,
			&member.Online,
			&created,
			&member.ServersCount,
		)
		if err != nil {
			return nil, trace.ErrorAborted(err, h.Log)
		}

		member.Role = userpb.Role_user // TODO: We transfer from the old format to the new one due to PHP version of the site
		member.Created = timestamppb.New(created.Time)
		response.Members = append(response.Members, member)
	}
	defer rows.Close()

	// Total count for pagination
	err = h.DB.Conn.QueryRowContext(ctx, `SELECT COUNT (*)
		FROM "project_member"
			INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
			INNER JOIN "user" AS "member" ON "project_member"."user_id" = "member"."id"
			INNER JOIN "user" AS "owner" ON "project"."owner_id" = "owner"."id"`+sqlSearch,
	).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	return response, nil
}

// ProjectMember is ...
func (h *Handler) ProjectMember(ctx context.Context, in *memberpb.ProjectMember_Request) (*memberpb.ProjectMember_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	var role string // TODO The old Role format used in PHP.Transfer to digital
	var created pgtype.Timestamp
	response := new(memberpb.ProjectMember_Response)

	err := h.DB.Conn.QueryRowContext(ctx, `SELECT
			"owner"."name" AS "owner_name",
			"project"."title" AS "project_name",
			"project_member"."user_id" AS "user_id",
			"member"."name" AS "user_name",
			"project_member"."role" AS "member_role",
			"project_member"."active" AS "member_active",
			"project_member"."created" AS "member_created"
		FROM "project_member"
			INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
			INNER JOIN "user" AS "member" ON "project_member"."user_id" = "member"."id"
			INNER JOIN "user" AS "owner" ON "project"."owner_id" = "owner"."id"
		WHERE "project_member"."id" = $1
			AND "project"."owner_id" = $2
			AND "project"."id" = $3`,
		in.GetMemberId(),
		in.GetOwnerId(),
		in.GetProjectId(),
	).Scan(&response.OwnerName,
		&response.ProjectName,
		&response.UserId,
		&response.UserName,
		&role,
		&response.Active,
		&created,
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	response.Role = userpb.Role(userpb.Role_value[role]) // TODO Old Role format
	response.MemberId = in.GetMemberId()
	response.OwnerId = in.GetOwnerId()
	response.ProjectId = in.GetProjectId()
	response.Created = timestamppb.New(created.Time)

	return response, nil
}

// AddProjectMember is ...
func (h *Handler) AddProjectMember(ctx context.Context, in *memberpb.AddProjectMember_Request) (*memberpb.AddProjectMember_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	response := new(memberpb.AddProjectMember_Response)

	err := h.DB.Conn.QueryRowContext(ctx, `INSERT INTO "project_member" ("project_id", "user_id", "role", "active")
		VALUES ($1, $2, $3, $4)
		RETURNING "id"`,
		in.GetProjectId(),
		in.GetUserId(),
		in.GetRole(),
		in.GetActive(),
	).Scan(&response.MemberId)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToAdd)
	}

	return response, nil
}

// UpdateProjectMember is ...
func (h *Handler) UpdateProjectMember(ctx context.Context, in *memberpb.UpdateProjectMember_Request) (*memberpb.UpdateProjectMember_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	var err error
	response := new(memberpb.UpdateProjectMember_Response)

	switch in.GetSetting().(type) {
	case *memberpb.UpdateProjectMember_Request_Role:
		_, err = h.DB.Conn.ExecContext(ctx, `UPDATE "project_member" SET "role" = $1, "last_update" = NOW() WHERE "id" = $2`,
			in.GetRole(),
			in.GetMemberId(),
		)

	case *memberpb.UpdateProjectMember_Request_Active:
		_, err = h.DB.Conn.ExecContext(ctx, `UPDATE "project_member" SET "active" = $1, "last_update" = NOW() WHERE "id" = $2`,
			in.GetActive(),
			in.GetMemberId(),
		)
	default:
		return nil, trace.Error(codes.Aborted, trace.MsgInvalidArgument)
	}

	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToUpdate)
	}

	return response, nil
}

// DeleteProjectMember is ...
func (h *Handler) DeleteProjectMember(ctx context.Context, in *memberpb.DeleteProjectMember_Request) (*memberpb.DeleteProjectMember_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	response := new(memberpb.DeleteProjectMember_Response)

	_, err := h.DB.Conn.ExecContext(ctx, `DELETE FROM "project_member" WHERE "id" = $1`,
		in.GetMemberId(),
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToDelete)
	}

	return response, nil
}

// UsersWithoutProject is ...
func (h *Handler) UsersWithoutProject(ctx context.Context, in *memberpb.UsersWithoutProject_Request) (*memberpb.UsersWithoutProject_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	response := new(memberpb.UsersWithoutProject_Response)

	rows, err := h.DB.Conn.QueryContext(ctx, `SELECT "id", "login", "email" FROM "user"
		WHERE "id" NOT IN(SELECT "user_id" FROM "project_member" WHERE "project_id" = $1)
			AND LOWER("login") LIKE LOWER($2 || '%')
		ORDER BY "login" ASC LIMIT 15 OFFSET 0`,
		in.GetProjectId(),
		in.GetLogin(),
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	for rows.Next() {
		user := new(memberpb.UsersWithoutProject_User)
		if err = rows.Scan(&user.UserId, &user.Login, &user.Email); err != nil {
			return nil, trace.ErrorAborted(err, h.Log)
		}

		response.Users = append(response.Users, user)
	}
	defer rows.Close()

	return response, nil
}

// ListMembersInvite is ...
func (h *Handler) ListMembersInvite(ctx context.Context, in *memberpb.ListMembersInvite_Request) (*memberpb.ListMembersInvite_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	response := new(memberpb.ListMembersInvite_Response)

	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := h.DB.Conn.QueryContext(ctx, `SELECT "id", "name", "surname", "email", "created", "status"
		FROM "project_invite"
		WHERE "project_invite"."project_id" = $1`+sqlFooter,
		in.GetProjectId(),
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	for rows.Next() {
		var created pgtype.Timestamp
		invite := new(memberpb.ListMembersInvite_Invites)
		err = rows.Scan(&invite.Id,
			&invite.Name,
			&invite.Surname,
			&invite.Email,
			&created,
			&invite.Status,
		)
		if err != nil {
			return nil, trace.ErrorAborted(err, h.Log)
		}

		invite.Created = timestamppb.New(created.Time)
		response.Invites = append(response.Invites, invite)
	}
	defer rows.Close()

	// Total count for pagination
	err = h.DB.Conn.QueryRowContext(ctx, `SELECT COUNT (*)
		FROM "project_invite"
		WHERE "project_invite"."project_id" = $1`,
		in.GetProjectId(),
	).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	return response, nil
}

// AddMemberInvite is ...
func (h *Handler) AddMemberInvite(ctx context.Context, in *memberpb.AddMemberInvite_Request) (*memberpb.AddMemberInvite_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	var inviteID string
	response := new(memberpb.AddMemberInvite_Response)

	// We check whether the user is invited with such an email to the project
	err := h.DB.Conn.QueryRowContext(ctx, `SELECT "id" FROM "project_invite" WHERE "email" = $1`,
		in.GetEmail(),
	).Scan(&inviteID)
	if err != nil && err != sql.ErrNoRows {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	if inviteID != "" {
		return nil, trace.Error(codes.AlreadyExists) // Email in use
	}

	err = h.DB.Conn.QueryRowContext(ctx, `INSERT INTO "project_invite" ("project_id", "email", "name", "surname", "invite", "status", "ldap_user")
		VALUES ($1, $2, $3, $4, $5, 'send', false)
		RETURNING "invite"`,
		in.GetProjectId(),
		in.GetEmail(),
		in.GetUserName(),
		in.GetUserSurname(),
		uuid.New().String(),
	).Scan(&response.Invite)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToAdd)
	}

	return response, nil
}

// DeleteMemberInvite is ...
func (h *Handler) DeleteMemberInvite(ctx context.Context, in *memberpb.DeleteMemberInvite_Request) (*memberpb.DeleteMemberInvite_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	response := new(memberpb.DeleteMemberInvite_Response)

	_, err := h.DB.Conn.ExecContext(ctx, `DELETE FROM "project_invite" WHERE "id" = $1 AND "project_id" = $2 AND "status" = 'send'`,
		in.GetInviteId(),
		in.GetProjectId(),
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToDelete)
	}

	return response, nil
}

// MemberInviteActivate is ...
func (h *Handler) MemberInviteActivate(ctx context.Context, in *memberpb.MemberInviteActivate_Request) (*memberpb.MemberInviteActivate_Response, error) {
	var userID, memberID, status string
	response := new(memberpb.MemberInviteActivate_Response)

	err := h.DB.Conn.QueryRowContext(ctx, `SELECT "user"."id", "project_invite"."project_id", "project_invite"."status"
		FROM "project_invite"
			INNER JOIN "user" ON "project_invite"."email" = "user"."email"
		WHERE "project_invite"."invite" = $1`, in.GetInvite(),
	).Scan(&userID,
		&response.ProjectId,
		&status,
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	if status == "" {
		return nil, trace.Error(codes.NotFound, trace.MsgInviteIsInvalid)
	}
	if userID == "" {
		return nil, trace.Error(codes.NotFound)
	}
	if userID != in.GetUserId() {
		return nil, trace.Error(codes.Aborted, trace.MsgAccessIsDeniedUser)
	}
	if status == "activated" {
		return nil, trace.Error(codes.NotFound, trace.MsgInviteIsActivated)
	}

	tx, err := h.DB.Conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgTransactionCreateError)
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `INSERT INTO "project_member" ("project_id","user_id","role")
		VALUES ($1, $2, 'user')
    RETURNING "id"`,
		response.ProjectId,
		userID,
	).Scan(&memberID)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToAdd)
	}

	_, err = tx.ExecContext(ctx, `UPDATE "project_invite" SET "status" = 'activated', "user_id" = $1, "last_update" = NOW() WHERE "invite" = $2`,
		in.GetUserId(),
		in.GetInvite(),
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToUpdate)
	}

	if err := tx.Commit(); err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgTransactionCommitError)
	}

	return response, nil
}

// ListServerMembers is ...
func (h *Handler) ListServerMembers(ctx context.Context, in *memberpb.ListServerMembers_Request) (*memberpb.ListServerMembers_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	response := new(memberpb.ListServerMembers_Response)

	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := h.DB.Conn.QueryContext(ctx, `SELECT
			"user"."id",
			"user"."login",
      "user"."name",
      "user"."surname",
			"user"."email",
			"server_member"."id",
			"server_member"."active",
			"server_member"."online",
			"server_member"."last_update"
		FROM "server_member"
			INNER JOIN "project_member" ON "server_member"."member_id" = "project_member"."id"
			INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
		WHERE "project_member"."project_id" = $1
			AND "server_member"."server_id" = $2`+sqlFooter,
		in.GetProjectId(),
		in.GetServerId(),
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	for rows.Next() {
		var lastUpdate pgtype.Timestamp
		member := new(memberpb.ServerMember_Response)
		err = rows.Scan(&member.UserId,
			&member.UserLogin,
			&member.UserName,
			&member.UserSurname,
			&member.Email,
			&member.MemberId,
			&member.Active,
			&member.Online,
			&lastUpdate,
		)
		if err != nil {
			return nil, trace.ErrorAborted(err, h.Log)
		}

		member.LastUpdate = timestamppb.New(lastUpdate.Time)
		response.Members = append(response.Members, member)
	}
	defer rows.Close()

	// Total members
	err = h.DB.Conn.QueryRowContext(ctx, `SELECT COUNT (*)
		FROM "server_member"
			INNER JOIN "project_member" ON "server_member"."member_id" = "project_member"."id"
			INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
		WHERE "project_member"."project_id" = $1
			AND "server_member"."server_id" = $2`,
		in.GetProjectId(),
		in.GetServerId(),
	).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	return response, nil
}

// ServerMember is ...
func (h *Handler) ServerMember(ctx context.Context, in *memberpb.ServerMember_Request) (*memberpb.ServerMember_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	var lastUpdate pgtype.Timestamp
	response := new(memberpb.ServerMember_Response)

	err := h.DB.Conn.QueryRowContext(ctx, `SELECT
			"user"."id",
			"user"."login",
			"server_member"."active",
			"server_member"."online",
			"server_member"."last_update"
		FROM "server_member"
			INNER JOIN "project_member" ON "server_member"."member_id" = "project_member"."id"
			INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
		WHERE "server_member"."id" = $1`,
		in.GetMemberId(),
	).Scan(&response.UserId,
		&response.UserLogin,
		&response.Active,
		&response.Online,
		&lastUpdate,
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	response.MemberId = in.GetMemberId()
	response.LastUpdate = timestamppb.New(lastUpdate.Time)

	return response, nil
}

// AddServerMember is ...
func (h *Handler) AddServerMember(ctx context.Context, in *memberpb.AddServerMember_Request) (*memberpb.AddServerMember_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	response := new(memberpb.AddServerMember_Response)

	err := h.DB.Conn.QueryRowContext(ctx, `SELECT "id"
		FROM "server_member"
		WHERE "server_member"."server_id" = $1
			AND "server_member"."member_id" = $2`,
		in.GetServerId(),
		in.GetMemberId(),
	).Scan(&response.MemberId)
	if err != nil && err != sql.ErrNoRows {
		return nil, trace.ErrorAborted(err, h.Log)
	}
	if response.MemberId != "" {
		return response, nil
	}

	err = h.DB.Conn.QueryRowContext(ctx, `INSERT INTO "server_member" ("server_id","member_id","active")
		VALUES ($1, $2, $3) RETURNING "id"`,
		in.GetServerId(),
		in.GetMemberId(),
		in.GetActive(),
	).Scan(&response.MemberId)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToAdd)
	}

	return response, nil
}

// UpdateServerMember is ...
func (h *Handler) UpdateServerMember(ctx context.Context, in *memberpb.UpdateServerMember_Request) (*memberpb.UpdateServerMember_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	response := new(memberpb.UpdateServerMember_Response)

	var err error
	switch in.GetSetting().(type) {
	case *memberpb.UpdateServerMember_Request_Active:
		_, err = h.DB.Conn.ExecContext(ctx, `UPDATE "server_member" SET "active" = $1, "last_update" = NOW() WHERE "id" = $2 AND "server_id" = $3`,
			in.GetActive(),
			in.GetMemberId(),
			in.GetServerId(),
		)
	case *memberpb.UpdateServerMember_Request_Online:
		_, err = h.DB.Conn.ExecContext(ctx, `UPDATE "server_member" SET "online" = $1, "last_update" = NOW() WHERE "id" = $2 AND "server_id" = $3`,
			in.GetOnline(),
			in.GetMemberId(),
			in.GetServerId(),
		)
	default:
		return nil, trace.Error(codes.Aborted, trace.MsgInvalidArgument)
	}

	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToUpdate)
	}

	return response, nil
}

// DeleteServerMember is ...
func (h *Handler) DeleteServerMember(ctx context.Context, in *memberpb.DeleteServerMember_Request) (*memberpb.DeleteServerMember_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	response := new(memberpb.DeleteServerMember_Response)

	_, err := h.DB.Conn.ExecContext(ctx, `DELETE FROM "server_member" WHERE "id" = $1 AND "server_id" = $2`,
		in.GetMemberId(),
		in.GetServerId(),
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log, trace.MsgFailedToDelete)
	}

	return response, nil
}

// MembersWithoutServer is ...
func (h *Handler) MembersWithoutServer(ctx context.Context, in *memberpb.MembersWithoutServer_Request) (*memberpb.MembersWithoutServer_Response, error) {
	if !project.IsOwnerProject(ctx, h.DB, in.GetProjectId(), in.GetOwnerId()) {
		return nil, trace.Error(codes.NotFound)
	}

	response := new(memberpb.MembersWithoutServer_Response)

	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := h.DB.Conn.QueryContext(ctx, `SELECT
			"project_member"."id",
			"user"."login",
			"user"."email",
      "user"."name",
      "user"."surname",
			"project_member"."role" AS "member_role",
			"project_member"."active" AS "member_active",
			"project_member"."online" AS "member_online"
		FROM "project_member"
			INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
		WHERE "project_member"."id" NOT IN(SELECT "member_id" FROM "server_member" WHERE "server_id" = $1)
			AND "project_member"."project_id" = $2
			AND LOWER("user"."login") LIKE LOWER($3 || '%') `+sqlFooter,
		in.GetServerId(),
		in.GetProjectId(),
		in.GetLogin(),
	)
	if err != nil {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	for rows.Next() {
		var role string
		member := new(memberpb.ServerMember_Response)
		err = rows.Scan(&member.MemberId,
			&member.UserLogin,
			&member.Email,
			&member.UserName,
			&member.UserSurname,
			&role,
			&member.Active,
			&member.Online,
		)
		if err != nil {
			return nil, trace.ErrorAborted(err, h.Log)
		}

		member.Role = userpb.Role(userpb.Role_value[role])
		response.Members = append(response.Members, member)
	}
	defer rows.Close()

	// Total count for pagination
	err = h.DB.Conn.QueryRowContext(ctx, `SELECT COUNT (*)
		FROM "project_member"
			INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
		WHERE "project_member"."id" NOT IN(SELECT "member_id" FROM "server_member" WHERE "server_id" = $1)
			AND "project_member"."project_id" = $2
			AND LOWER("user"."login") LIKE LOWER($3 || '%')`,
		in.GetServerId(),
		in.GetProjectId(),
		in.GetLogin(),
	).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		return nil, trace.ErrorAborted(err, h.Log)
	}

	return response, nil
}
