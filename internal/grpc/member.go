package grpc

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	pb_member "github.com/werbot/werbot/api/proto/member"
	pb_user "github.com/werbot/werbot/api/proto/user"
)

type member struct {
	pb_member.UnimplementedMemberHandlersServer
}

// ListProjectMembers is ...
func (m *member) ListProjectMembers(ctx context.Context, in *pb_member.ListProjectMembers_Request) (*pb_member.ListProjectMembers_Response, error) {
	sqlSearch := service.db.SQLAddWhere(in.GetQuery())
	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := service.db.Conn.Query(`SELECT
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
			( SELECT COUNT ( * ) FROM "server_member" WHERE "member_id" = "project_member"."id"  ) AS "count_servers"
		FROM
			"project_member"
			INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
			INNER JOIN "user" AS "member" ON "project_member"."user_id" = "member"."id"
			INNER JOIN "user" AS "owner" ON "project"."owner_id" = "owner"."id"` + sqlSearch + sqlFooter)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToSelect
	}

	members := []*pb_member.ProjectMember_Response{}
	for rows.Next() {
		var role string
		var created pgtype.Timestamp
		member := new(pb_member.ProjectMember_Response)
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
			service.log.FromGRPC(err).Send()
			return nil, errFailedToScan
		}
		member.Role = pb_user.RoleUser_USER // TODO: We transfer from the old format to the new one due to PHP version of the site
		member.Created = timestamppb.New(created.Time)
		members = append(members, member)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	err = service.db.Conn.QueryRow(`SELECT
      COUNT (*)
		FROM
			"project_member"
			INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
			INNER JOIN "user" AS "member" ON "project_member"."user_id" = "member"."id"
			INNER JOIN "user" AS "owner" ON "project"."owner_id" = "owner"."id"` + sqlSearch).
		Scan(&total)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToScan
	}

	return &pb_member.ListProjectMembers_Response{
		Total:   total,
		Members: members,
	}, nil
}

// ProjectMember is ...
func (m *member) ProjectMember(ctx context.Context, in *pb_member.ProjectMember_Request) (*pb_member.ProjectMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	var role string // TODO The old Role format used in PHP.Transfer to digital
	var created pgtype.Timestamp
	member := new(pb_member.ProjectMember_Response)
	err := service.db.Conn.QueryRow(`SELECT
			"owner"."name" AS "owner_name",
			"project"."title" AS "project_name",
			"project_member"."user_id" AS "user_id",
			"member"."name" AS "user_name",
			"project_member"."role" AS "member_role",
			"project_member"."active" AS "member_active",
			"project_member"."created" AS "member_created"
		FROM
			"project_member"
			INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
			INNER JOIN "user" AS "member" ON "project_member"."user_id" = "member"."id"
			INNER JOIN "user" AS "owner" ON "project"."owner_id" = "owner"."id"
		WHERE
			"project_member"."id" = $1
			AND "project"."owner_id" = $2
			AND "project"."id" = $3`, in.GetMemberId(), in.GetOwnerId(), in.GetProjectId()).
		Scan(
			&member.OwnerName,
			&member.ProjectName,
			&member.UserId,
			&member.UserName,
			&role,
			&member.Active,
			&created,
		)
	if err != nil {
		service.log.FromGRPC(err).Send()
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		return nil, errFailedToScan
	}

	member.Role = pb_user.RoleUser(pb_user.RoleUser_value[role]) // TODO Old Role format
	member.MemberId = in.GetMemberId()
	member.OwnerId = in.GetOwnerId()
	member.ProjectId = in.GetProjectId()
	member.Created = timestamppb.New(created.Time)
	return member, nil
}

// AddProjectMember is ...
func (m *member) AddProjectMember(ctx context.Context, in *pb_member.AddProjectMember_Request) (*pb_member.AddProjectMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	var id, ownerName, projectName, userName string
	var created pgtype.Timestamp
	getMember, err := m.MemberByID(ctx, &pb_member.MemberByID_Request{
		UserId:    in.GetUserId(),
		ProjectId: in.GetProjectId(),
	})
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, err
	}
	if getMember.Status.Value {
		return nil, errors.New("The user exists in the given project")
	}

	err = service.db.Conn.QueryRow(`INSERT
    INTO "project_member" (
			"project_id",
			"user_id",
			"role",
			"created",
			"active"
		)
		VALUES
			($1, $2, $3, NOW( ), $4)
		RETURNING "id", "created"`,
		in.GetProjectId(),
		in.GetUserId(),
		in.GetRole(),
		in.GetActive(),
	).Scan(&id, &created)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	err = service.db.Conn.QueryRow(`SELECT
			"owner"."name" AS "owner_name",
			"project"."title" AS "project_name",
			"member"."name" AS "member_name"
		FROM
			"project_member"
			INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
			INNER JOIN "user" AS "owner" ON "project"."owner_id" = "owner"."id"
			INNER JOIN "user" AS "member" ON "project_member"."user_id" = "member"."id"
		WHERE
			"project_member"."id" = $1`, id).
		Scan(
			&ownerName,
			&projectName,
			&userName,
		)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToScan
	}

	return &pb_member.AddProjectMember_Response{
		MemberId: id,
	}, nil
}

// UpdateProjectMember is ...
func (m *member) UpdateProjectMember(ctx context.Context, in *pb_member.UpdateProjectMember_Request) (*pb_member.UpdateProjectMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	var ownerName, projectName, userName string
	var created pgtype.Timestamp
	data, err := service.db.Conn.Exec(`UPDATE
      "project_member"
		SET
			"role" = $1,
			"active" = $2
		WHERE
			"id" = $3`,
		in.GetRole(),
		in.GetActive(),
		in.GetMemberId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	err = service.db.Conn.QueryRow(`SELECT
			"owner"."name" AS owner_name,
			"project"."title" AS project_name,
			"member"."name" AS user_name,
			"project_member"."created" AS member_created
		FROM
			"project_member"
			INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
			INNER JOIN "user" AS "member" ON "project_member"."project_id" = "member"."id"
			INNER JOIN "user" AS "owner" ON "project"."owner_id" = "owner"."id"
		WHERE
			"project_member"."id" = $1`,
		in.GetMemberId(),
	).Scan(
		&ownerName,
		&projectName,
		&userName,
		&created,
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToScan
	}

	return &pb_member.UpdateProjectMember_Response{}, nil
}

// DeleteProjectMember is ...
func (m *member) DeleteProjectMember(ctx context.Context, in *pb_member.DeleteProjectMember_Request) (*pb_member.DeleteProjectMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	data, err := service.db.Conn.Exec(`DELETE FROM "project_member" WHERE "id" = $1`, in.GetMemberId())
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_member.DeleteProjectMember_Response{}, nil
}

// UpdateProjectMemberStatus is ...
func (m *member) UpdateProjectMemberStatus(ctx context.Context, in *pb_member.UpdateProjectMemberStatus_Request) (*pb_member.UpdateProjectMemberStatus_Response, error) {
	// TODO After turning off, turn off all users who online
	data, err := service.db.Conn.Exec(`UPDATE
      "project_member"
		SET
			"active" = $1
		FROM
			"project"
		WHERE
			"project_member"."id" = $2 AND
			"project"."owner_id"  = $3 AND
			"project_member"."project_id" = "project"."id"`,
		in.GetStatus(),
		in.GetMemberId(),
		in.GetOwnerId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if rows, _ := data.RowsAffected(); rows != 1 {
		return nil, errNotFound
	}

	return &pb_member.UpdateProjectMemberStatus_Response{}, nil
}

// MemberByID is ...
func (m *member) MemberByID(ctx context.Context, in *pb_member.MemberByID_Request) (*pb_member.MemberByID_Response, error) {
	count := 0
	member := new(pb_member.MemberByID_Response)
	err := service.db.Conn.QueryRow(`SELECT
      COUNT (*)
		FROM
			"project_member"
		WHERE
			"project_member"."user_id" = $1
			AND "project_member"."project_id" = $2`,
		in.GetUserId(),
		in.GetProjectId(),
	).Scan(&count)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToScan
	}
	if count > 0 {
		member.Status = &wrapperspb.BoolValue{
			Value: true,
		}
		return member, nil
	}

	member.Status = &wrapperspb.BoolValue{
		Value: false,
	}
	return member, nil
}

// UsersByName is ...
func (m *member) UsersByName(ctx context.Context, in *pb_member.UsersByName_Request) (*pb_member.UsersByName_Response, error) {
	users := []*pb_member.UsersByName_Response_SearchUsersResult{}
	rows, err := service.db.Conn.Query(`SELECT
			"id" AS "member_id",
			"name" AS "member_name",
			"email"
		FROM
			"user"
		WHERE
			"user"."name"
		LIKE
			'$1%'
		ORDER BY "name" ASC
		LIMIT 15 OFFSET 0`,
		in.Name,
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToSelect
	}

	for rows.Next() {
		user := new(pb_member.UsersByName_Response_SearchUsersResult)
		err = rows.Scan(
			&user.MemberId,
			&user.MemberName,
			&user.Email,
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToScan
		}
		users = append(users, user)
	}
	defer rows.Close()

	return &pb_member.UsersByName_Response{
		Users: users,
	}, nil
}

// UsersWithoutProject
func (m *member) UsersWithoutProject(ctx context.Context, in *pb_member.UsersWithoutProject_Request) (*pb_member.UsersWithoutProject_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	users := []*pb_member.UsersWithoutProject_Response_User{}
	rows, err := service.db.Conn.Queryx(`SELECT
			"id",
			"name",
			"email"
		FROM
			"user"
		WHERE
			"id" NOT IN(SELECT "user_id" FROM "project_member" WHERE "project_id" = $1)
			AND LOWER("name") LIKE LOWER($2 || '%')
		ORDER BY
			"name" ASC
		LIMIT 15 OFFSET 0`,
		in.GetProjectId(),
		in.GetName(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToSelect
	}

	for rows.Next() {
		user := new(pb_member.UsersWithoutProject_Response_User)
		err = rows.Scan(
			&user.UserId,
			&user.Name,
			&user.Email,
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToScan
		}
		users = append(users, user)
	}
	defer rows.Close()

	return &pb_member.UsersWithoutProject_Response{
		Users: users,
	}, nil
}

// ListServerMembers is ...
func (m *member) ListServerMembers(ctx context.Context, in *pb_member.ListServerMembers_Request) (*pb_member.ListServerMembers_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	members := []*pb_member.ServerMember_Response{}
	rows, err := service.db.Conn.Query(`SELECT
			"user"."id",
			"user"."name",
			"user"."email",
			"server_member"."id",
			"server_member"."active",
			"server_member"."online",
			"server_member"."last_activity"
		FROM
			"server_member"
			INNER JOIN "project_member" ON "server_member"."member_id" = "project_member"."id"
			INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
		WHERE
			"project_member"."project_id" = $1
			AND "server_member"."server_id" = $2`+sqlFooter,
		in.GetProjectId(),
		in.GetServerId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToSelect
	}

	for rows.Next() {
		var lastActivity pgtype.Timestamp
		member := new(pb_member.ServerMember_Response)
		err = rows.Scan(
			&member.UserId,
			&member.UserName,
			&member.Email,
			&member.MemberId,
			&member.Active,
			&member.Online,
			&lastActivity,
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToScan
		}
		member.LastActivity = timestamppb.New(lastActivity.Time)
		members = append(members, member)
	}
	defer rows.Close()

	var total int32
	err = service.db.Conn.QueryRow(`SELECT
			COUNT (*)
		FROM
			"server_member"
			INNER JOIN "project_member" ON "server_member"."member_id" = "project_member"."id"
			INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
		WHERE
			"project_member"."project_id" = $1
			AND "server_member"."server_id" = $2`,
		in.GetProjectId(),
		in.GetServerId(),
	).Scan(&total)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToScan
	}

	return &pb_member.ListServerMembers_Response{
		Total:   total,
		Members: members,
	}, nil
}

// ServerMember is ...
func (m *member) ServerMember(ctx context.Context, in *pb_member.ServerMember_Request) (*pb_member.ServerMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	var lastActivity pgtype.Timestamp
	member := new(pb_member.ServerMember_Response)
	err := service.db.Conn.QueryRowx(`SELECT
			"user"."id",
			"user"."name",
			"server_member"."active",
			"server_member"."online",
			"server_member"."last_activity"
		FROM
			"server_member"
			INNER JOIN "project_member" ON "server_member"."member_id" = "project_member"."id"
			INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
		WHERE
			"server_member"."id" = $1`,
		in.GetMemberId()).
		Scan(
			&member.UserId,
			&member.UserName,
			&member.Active,
			&member.Online,
			&lastActivity,
		)
	if err != nil {
		service.log.FromGRPC(err).Send()
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		return nil, errFailedToScan
	}

	member.MemberId = in.GetMemberId()
	member.LastActivity = timestamppb.New(lastActivity.Time)
	return member, nil
}

// AddServerMember is ...
func (m *member) AddServerMember(ctx context.Context, in *pb_member.AddServerMember_Request) (*pb_member.AddServerMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	var memberID string
	member := new(pb_member.AddServerMember_Response)
	err := service.db.Conn.QueryRow(`SELECT
			"id"
		FROM
			"server_member"
		WHERE
			"server_member"."server_id" = $1
			AND "server_member"."member_id" = $2`,
		in.GetServerId(),
		in.GetMemberId(),
	).Scan(&memberID)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToScan
	}
	if memberID != "" {
		member.MemberId = memberID
		return member, nil
	}

	err = service.db.Conn.QueryRow(`INSERT
		INTO "server_member" (
			"server_id",
			"member_id",
			"active"
		)
		VALUES ($1, $2, $3)
		RETURNING "id"`,
		in.GetServerId(),
		in.GetMemberId(),
		in.GetActive(),
	).Scan(&memberID)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	member.MemberId = memberID
	return member, nil
}

// UpdateServerMember is ...
func (m *member) UpdateServerMember(ctx context.Context, in *pb_member.UpdateServerMember_Request) (*pb_member.UpdateServerMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	data, err := service.db.Conn.Exec(`UPDATE
        "server_member"
			SET
				"active" = $1
			WHERE
				"id" = $2
				AND "server_id" = $3`,
		in.GetActive(),
		in.GetMemberId(),
		in.GetServerId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_member.UpdateServerMember_Response{}, nil
}

// DeleteServerMember is ...
func (m *member) DeleteServerMember(ctx context.Context, in *pb_member.DeleteServerMember_Request) (*pb_member.DeleteServerMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	data, err := service.db.Conn.Exec(`DELETE
			FROM
				"server_member"
			WHERE
				"id" = $1
				AND "server_id" = $2`,
		in.GetMemberId(),
		in.GetServerId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_member.DeleteServerMember_Response{}, nil
}

// UpdateServerMemberStatus is ...
func (m *member) UpdateServerMemberStatus(ctx context.Context, in *pb_member.UpdateServerMemberStatus_Request) (*pb_member.UpdateServerMemberStatus_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	data, err := service.db.Conn.Exec(`UPDATE
        "server_member"
			SET
				"active" = $1
			WHERE
				"server_member"."id" = $2
				AND "server_member"."server_id" = $3`,
		in.GetStatus(),
		in.GetMemberId(),
		in.GetServerId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToDelete
	}
	if rows, _ := data.RowsAffected(); rows != 1 {
		return nil, errNotFound
	}

	return &pb_member.UpdateServerMemberStatus_Response{}, nil
}

// MembersWithoutServer
func (m *member) MembersWithoutServer(ctx context.Context, in *pb_member.MembersWithoutServer_Request) (*pb_member.MembersWithoutServer_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := service.db.Conn.Queryx(`SELECT
			"project_member"."id",
			"user"."name",
			"user"."email",
			"project_member"."role" AS "member_role",
			"project_member"."active" AS "member_active",
			"project_member"."online" AS "member_online"
		FROM
			"project_member"
			INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
		WHERE
			"project_member"."id" NOT IN(SELECT "member_id" FROM "server_member" WHERE "server_id" = $1)
			AND "project_member"."project_id" = $2
			AND LOWER("user"."name") LIKE LOWER($3 || '%') `+sqlFooter,
		in.GetServerId(),
		in.GetProjectId(),
		in.GetName(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToSelect
	}

	var role string
	members := []*pb_member.ServerMember_Response{}
	for rows.Next() {
		member := new(pb_member.ServerMember_Response)
		err = rows.Scan(
			&member.MemberId,
			&member.UserName,
			&member.Email,
			&role,
			&member.Active,
			&member.Online,
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToScan
		}
		member.Role = pb_user.RoleUser(pb_user.RoleUser_value[role])
		members = append(members, member)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	err = service.db.Conn.QueryRow(`SELECT
			COUNT (*)
		FROM
			"project_member"
			INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
		WHERE
			"project_member"."id" NOT IN(SELECT "member_id" FROM "server_member" WHERE "server_id" = $1)
			AND "project_member"."project_id" = $2
			AND LOWER("user"."name") LIKE LOWER($3 || '%')`,
		in.GetServerId(),
		in.GetProjectId(),
		in.GetName(),
	).Scan(&total)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToScan
	}

	return &pb_member.MembersWithoutServer_Response{
		Total:   total,
		Members: members,
	}, nil
}

// ListProjectMembersInvite is ...
func (m *member) ListProjectMembersInvite(ctx context.Context, in *pb_member.ListProjectMembersInvite_Request) (*pb_member.ListProjectMembersInvite_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := service.db.Conn.Query(`SELECT
			"id",
			"name",
			"surname",
			"email",
			"created",
			"status"
		FROM
			"project_invite"
		WHERE
			"project_invite"."project_id" = $1`+sqlFooter,
		in.GetProjectId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToSelect
	}

	invites := []*pb_member.ListProjectMembersInvite_Invites{}
	for rows.Next() {
		var created pgtype.Timestamp
		invite := new(pb_member.ListProjectMembersInvite_Invites)
		err = rows.Scan(
			&invite.Id,
			&invite.Name,
			&invite.Surname,
			&invite.Email,
			&created,
			&invite.Status,
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToScan
		}
		invite.Created = timestamppb.New(created.Time)
		invites = append(invites, invite)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	err = service.db.Conn.QueryRow(`SELECT
			COUNT (*)
		FROM
			"project_invite"
		WHERE
			"project_invite"."project_id" = $1`,
		in.GetProjectId(),
	).Scan(&total)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToScan
	}

	return &pb_member.ListProjectMembersInvite_Response{
		Total:   total,
		Invites: invites,
	}, nil
}

// AddProjectMemberInvite is ...
func (m *member) AddProjectMemberInvite(ctx context.Context, in *pb_member.AddProjectMemberInvite_Request) (*pb_member.AddProjectMemberInvite_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	var invite, inviteID string
	err := service.db.Conn.QueryRow(`SELECT
			"id"
		FROM
			"project_invite"
		WHERE
			"project_invite"."email" = $1`,
		in.GetEmail(),
	).Scan(&inviteID)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}
	if inviteID != "" {
		return nil, errObjectAlreadyExists // Email in use
	}

	err = service.db.Conn.QueryRow(`INSERT
		INTO "project_invite" (
			"project_id",
			"email",
			"name",
			"surname",
			"invite",
			"created",
			"status",
			"ldap_user"
		)
		VALUES
			($1, $2, $3, $4, $5, NOW( ), 'send', false)
		RETURNING "invite"`,
		in.GetProjectId(),
		in.GetEmail(),
		in.GetUserName(),
		in.GetUserSurname(),
		uuid.New().String(),
	).Scan(&invite)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	return &pb_member.AddProjectMemberInvite_Response{
		Invite: invite,
	}, nil
}

// DeleteProjectMemberInvite is ...
func (m *member) DeleteProjectMemberInvite(ctx context.Context, in *pb_member.DeleteProjectMemberInvite_Request) (*pb_member.DeleteProjectMemberInvite_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errNotFound
	}

	data, err := service.db.Conn.Exec(`DELETE
		FROM
			"project_invite"
		WHERE
			"id" = $1
			AND "project_id" = $2
			AND "status" = 'send'`,
		in.GetInviteId(),
		in.GetProjectId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_member.DeleteProjectMemberInvite_Response{}, nil
}

// ProjectMemberInviteActivate is ...
func (m *member) ProjectMemberInviteActivate(ctx context.Context, in *pb_member.ProjectMemberInviteActivate_Request) (*pb_member.ProjectMemberInviteActivate_Response, error) {
	var inviteID, userID, projectID, memberID, status string

	service.db.Conn.QueryRow(`SELECT
			"project_invite"."id"
		FROM
			"project_invite"
		WHERE
			"project_invite"."invite" = $1`,
		in.GetInvite(),
	).Scan(
		&inviteID,
	)

	if inviteID == "" {
		return nil, errInviteIsInvalid
	}

	service.db.Conn.QueryRow(`SELECT
			"user"."id",
			"project_invite"."project_id",
			"project_invite"."status"
		FROM
			"project_invite"
			INNER JOIN "user" ON "project_invite"."email" = "user"."email"
		WHERE
			"project_invite"."id" = $1`,
		inviteID,
	).Scan(
		&userID,
		&projectID,
		&status,
	)

	if userID == "" {
		return nil, errors.New("New user")
	}

	if userID != in.GetUserId() {
		return nil, errors.New("Wrong user")
	}

	if status == "activated" {
		return nil, errInviteIsActivated
	}

	err := service.db.Conn.QueryRow(`INSERT
		INTO "project_member" (
			"project_id",
			"user_id",
			"role",
			"created"
		)
		VALUES
			($1, $2, 'user', NOW( ))
		RETURNING "id"`,
		projectID,
		userID,
	).Scan(&memberID)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	data, err := service.db.Conn.Exec(`UPDATE
      "project_invite"
		SET
			"status" = 'activated',
			"user_id" = $1
		WHERE
			"invite" = $2`,
		in.GetUserId(),
		in.GetInvite(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_member.ProjectMemberInviteActivate_Response{
		ProjectId: projectID,
	}, nil
}
