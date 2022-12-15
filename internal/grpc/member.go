package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	pb_member "github.com/werbot/werbot/api/proto/member"
	pb_user "github.com/werbot/werbot/api/proto/user"
	"github.com/werbot/werbot/internal"
)

type member struct {
	pb_member.UnimplementedMemberHandlersServer
}

// ListProjectMembers is ...
func (m *member) ListProjectMembers(ctx context.Context, in *pb_member.ListProjectMembers_Request) (*pb_member.ListProjectMembers_Response, error) {
	sqlSearch := db.SQLAddWhere(in.GetQuery())
	sqlFooter := db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())

	rows, err := db.Conn.Query(`SELECT
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
		return nil, err
	}

	members := []*pb_member.GetProjectMember_Response{}
	for rows.Next() {
		member := pb_member.GetProjectMember_Response{}
		var created pgtype.Timestamp
		var role string

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
			return nil, err
		}

		member.Role = pb_user.RoleUser_USER // TODO: We transfer from the old format to the new one due to PHP version of the site
		member.Created = timestamppb.New(created.Time)
		members = append(members, &member)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	db.Conn.QueryRow(`SELECT COUNT (*)
		FROM
			"project_member"
			INNER JOIN "project" ON "project_member"."project_id" = "project"."id"
			INNER JOIN "user" AS "member" ON "project_member"."user_id" = "member"."id"
			INNER JOIN "user" AS "owner" ON "project"."owner_id" = "owner"."id"` + sqlSearch).Scan(&total)

	return &pb_member.ListProjectMembers_Response{
		Total:   total,
		Members: members,
	}, nil
}

// GetProjectMember is ...
func (m *member) GetProjectMember(ctx context.Context, in *pb_member.GetProjectMember_Request) (*pb_member.GetProjectMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	member := pb_member.GetProjectMember_Response{}

	// TODO старый формат ROLE, используемый в php. Перевести в цифровой
	var role string
	//

	var created pgtype.Timestamp
	err := db.Conn.QueryRow(`SELECT
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
		return nil, errors.New(internal.ErrNotFound)
	}

	// TODO старый формат ROLE
	member.Role = pb_user.RoleUser(pb_user.RoleUser_value[role])
	//

	member.MemberId = in.GetMemberId()
	member.OwnerId = in.GetOwnerId()
	member.ProjectId = in.GetProjectId()
	member.Created = timestamppb.New(created.Time)

	return &member, nil
}

// CreateProjectMember is ...
func (m *member) CreateProjectMember(ctx context.Context, in *pb_member.CreateProjectMember_Request) (*pb_member.CreateProjectMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	var id string
	var created pgtype.Timestamp
	var ownerName, projectName, userName string

	getMember, err := m.GetMemberByID(ctx, &pb_member.GetMemberByID_Request{
		UserId:    in.GetUserId(),
		ProjectId: in.GetProjectId(),
	})
	if err != nil {
		return nil, errors.New("User not found by given ID")
	}
	if getMember.Status.Value {
		return nil, errors.New("The user exists in the given project")
	}

	err = db.Conn.QueryRow(`INSERT INTO "project_member" (
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
		return nil, errors.New("CreateProjectMember failed")
	}

	err = db.Conn.QueryRow(`SELECT
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
		return nil, errors.New("Get member info failed")
	}

	return &pb_member.CreateProjectMember_Response{
		MemberId: id,
	}, nil
}

// UpdateProjectMember is ...
func (m *member) UpdateProjectMember(ctx context.Context, in *pb_member.UpdateProjectMember_Request) (*pb_member.UpdateProjectMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	var created pgtype.Timestamp
	var ownerName, projectName, userName string

	_, err := db.Conn.Exec(`UPDATE "project_member" 
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
		return nil, errors.New("UpdateProjectMember failed")
	}

	err = db.Conn.QueryRow(`SELECT
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
		return nil, errors.New("Get member info failed from UpdateMember")
	}

	return &pb_member.UpdateProjectMember_Response{}, nil
}

// DeleteProjectMember is ...
func (m *member) DeleteProjectMember(ctx context.Context, in *pb_member.DeleteProjectMember_Request) (*pb_member.DeleteProjectMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	_, err := db.Conn.Query(`DELETE FROM "project_member" WHERE "id" = $1`, in.GetMemberId())
	if err != nil {
		return &pb_member.DeleteProjectMember_Response{}, errors.New("DeleteMember failed")
	}

	return &pb_member.DeleteProjectMember_Response{}, nil
}

// UpdateProjectMemberStatus is ...
func (m *member) UpdateProjectMemberStatus(ctx context.Context, in *pb_member.UpdateProjectMemberStatus_Request) (*pb_member.UpdateProjectMemberStatus_Response, error) {
	// TODO After turning off, turn off all users who online
	ct, err := db.Conn.Exec(`	UPDATE "project_member"
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
		return &pb_member.UpdateProjectMemberStatus_Response{}, err
	}
	if rows, _ := ct.RowsAffected(); rows != 1 {
		return &pb_member.UpdateProjectMemberStatus_Response{}, errors.New(internal.ErrNotFound)
	}

	return &pb_member.UpdateProjectMemberStatus_Response{}, nil
}

// GetMemberByID is ...
func (m *member) GetMemberByID(ctx context.Context, in *pb_member.GetMemberByID_Request) (*pb_member.GetMemberByID_Response, error) {
	count := 0
	err := db.Conn.QueryRow(`SELECT COUNT (*) 
		FROM 
			"project_member" 
		WHERE 
			"project_member"."user_id" = $1 
			AND "project_member"."project_id" = $2`,
		in.GetUserId(),
		in.GetProjectId(),
	).Scan(&count)
	if err != nil {
		return nil, errors.New("GetMemberByID info failed")
	}

	if count > 0 {
		return &pb_member.GetMemberByID_Response{
			Status: &wrapperspb.BoolValue{
				Value: true,
			},
		}, nil
	}

	return &pb_member.GetMemberByID_Response{
		Status: &wrapperspb.BoolValue{
			Value: false,
		},
	}, nil
}

// GetUsersByName is ...
func (m *member) GetUsersByName(ctx context.Context, in *pb_member.GetUsersByName_Request) (*pb_member.GetUsersByName_Response, error) {
	users := []*pb_member.GetUsersByName_Response_SearchUsersResult{}

	rows, err := db.Conn.Query(`SELECT 
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
		return nil, errors.New("Action GetUsersByName query parameters failed")
	}

	for rows.Next() {
		user := pb_member.GetUsersByName_Response_SearchUsersResult{}

		err = rows.Scan(
			&user.MemberId,
			&user.MemberName,
			&user.Email,
		)

		if err != nil {
			return nil, errors.New("GetUsersByName scan failed")
		}

		users = append(users, &user)
	}
	defer rows.Close()

	return &pb_member.GetUsersByName_Response{
		Users: users,
	}, nil
}

// GetUsersWithoutProject
func (m *member) GetUsersWithoutProject(ctx context.Context, in *pb_member.GetUsersWithoutProject_Request) (*pb_member.GetUsersWithoutProject_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	users := []*pb_member.GetUsersWithoutProject_Response_User{}
	rows, err := db.Conn.Queryx(`SELECT
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
		return nil, errors.New("Action GetUsersWithoutProject query parameters failed")
	}

	for rows.Next() {
		user := pb_member.GetUsersWithoutProject_Response_User{}

		err = rows.Scan(
			&user.UserId,
			&user.Name,
			&user.Email,
		)

		if err != nil {
			return nil, errors.New("GetUsersWithoutProject scan failed")
		}

		users = append(users, &user)
	}
	defer rows.Close()

	return &pb_member.GetUsersWithoutProject_Response{
		Users: users,
	}, nil
}

// ListServerMembers is ...
func (m *member) ListServerMembers(ctx context.Context, in *pb_member.ListServerMembers_Request) (*pb_member.ListServerMembers_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	sqlFooter := db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())

	members := []*pb_member.GetServerMember_Response{}
	rows, err := db.Conn.Query(`SELECT
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
		return nil, errors.New("GetServerMembers: query parameters failed")
	}

	for rows.Next() {
		member := pb_member.GetServerMember_Response{}
		var lastActivity pgtype.Timestamp

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
			return nil, errors.New("GetServerMembers: scan failed")
		}

		member.LastActivity = timestamppb.New(lastActivity.Time)
		members = append(members, &member)
	}
	defer rows.Close()

	var total int32
	db.Conn.QueryRow(`SELECT COUNT (*)
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

	return &pb_member.ListServerMembers_Response{
		Total:   total,
		Members: members,
	}, nil
}

// GetServerMember is ...
func (m *member) GetServerMember(ctx context.Context, in *pb_member.GetServerMember_Request) (*pb_member.GetServerMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	member := pb_member.GetServerMember_Response{}

	var lastActivity pgtype.Timestamp
	err := db.Conn.QueryRowx(`SELECT
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
		in.GetMemberId(),
	).Scan(
		&member.UserId,
		&member.UserName,
		&member.Active,
		&member.Online,
		&lastActivity,
	)
	if err != nil {
		return nil, errors.New(internal.ErrNotFound)
	}

	member.MemberId = in.GetMemberId()
	member.LastActivity = timestamppb.New(lastActivity.Time)

	return &member, nil
}

// CreateServerMember is ...
func (m *member) CreateServerMember(ctx context.Context, in *pb_member.CreateServerMember_Request) (*pb_member.CreateServerMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	var memberID string
	db.Conn.QueryRow(`SELECT 
			"id"
		FROM 
			"server_member" 
		WHERE
			"server_member"."server_id" = $1 
			AND "server_member"."member_id" = $2`,
		in.GetServerId(),
		in.GetMemberId(),
	).Scan(&memberID)
	if memberID != "" {
		return &pb_member.CreateServerMember_Response{
			MemberId: memberID,
		}, nil
	}

	err := db.Conn.QueryRow(`INSERT 
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
		return nil, errors.New("CreateMember failed")
	}

	return &pb_member.CreateServerMember_Response{
		MemberId: memberID,
	}, nil
}

// UpdateServerMember is ...
func (m *member) UpdateServerMember(ctx context.Context, in *pb_member.UpdateServerMember_Request) (*pb_member.UpdateServerMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	_, err := db.Conn.Exec(`UPDATE "server_member" 
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
		return nil, errors.New("UpdateMember failed")
	}

	return &pb_member.UpdateServerMember_Response{}, nil
}

// DeleteServerMember is ...
func (m *member) DeleteServerMember(ctx context.Context, in *pb_member.DeleteServerMember_Request) (*pb_member.DeleteServerMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	_, err := db.Conn.Exec(`DELETE 
			FROM 
				"server_member" 
			WHERE 
				"id" = $1 
				AND "server_id" = $2`,
		in.GetMemberId(),
		in.GetServerId(),
	)
	if err != nil {
		return &pb_member.DeleteServerMember_Response{}, errors.New("DeleteMember failed")
	}

	return &pb_member.DeleteServerMember_Response{}, nil
}

// UpdateServerMemberStatus is ...
func (m *member) UpdateServerMemberStatus(ctx context.Context, in *pb_member.UpdateServerMemberStatus_Request) (*pb_member.UpdateServerMemberStatus_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	ct, err := db.Conn.Exec(`UPDATE "server_member"
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
		return &pb_member.UpdateServerMemberStatus_Response{}, err
	}
	if rows, _ := ct.RowsAffected(); rows != 1 {
		return &pb_member.UpdateServerMemberStatus_Response{}, errors.New(internal.ErrNotFound)
	}

	return &pb_member.UpdateServerMemberStatus_Response{}, nil
}

// GetMembersWithoutServer
func (m *member) GetMembersWithoutServer(ctx context.Context, in *pb_member.GetMembersWithoutServer_Request) (*pb_member.GetMembersWithoutServer_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	sqlFooter := db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())

	rows, err := db.Conn.Queryx(`SELECT
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
		return nil, errors.New("GetUsersWithoutServer: query parameters failed")
	}

	var role string
	members := []*pb_member.GetServerMember_Response{}
	for rows.Next() {
		member := pb_member.GetServerMember_Response{}

		err = rows.Scan(
			&member.MemberId,
			&member.UserName,
			&member.Email,
			&role,
			&member.Active,
			&member.Online,
		)
		if err != nil {
			return nil, errors.New("GetUsersWithoutServer: scan failed")
		}

		member.Role = pb_user.RoleUser(pb_user.RoleUser_value[role])
		members = append(members, &member)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	db.Conn.QueryRow(`SELECT COUNT (*)
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

	return &pb_member.GetMembersWithoutServer_Response{
		Total:   total,
		Members: members,
	}, nil
}

// ListProjectMembersInvite is ...
func (m *member) ListProjectMembersInvite(ctx context.Context, in *pb_member.ListProjectMembersInvite_Request) (*pb_member.ListProjectMembersInvite_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	sqlFooter := db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())

	rows, err := db.Conn.Query(`SELECT
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
		return nil, err
	}

	invites := []*pb_member.ListProjectMembersInvite_Invites{}
	for rows.Next() {
		invite := pb_member.ListProjectMembersInvite_Invites{}
		var created pgtype.Timestamp

		err = rows.Scan(
			&invite.Id,
			&invite.Name,
			&invite.Surname,
			&invite.Email,
			&created,
			&invite.Status,
		)
		if err != nil {
			return nil, err
		}

		invite.Created = timestamppb.New(created.Time)
		invites = append(invites, &invite)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	db.Conn.QueryRow(`SELECT COUNT (*)
		FROM
			"project_invite"
		WHERE
			"project_invite"."project_id" = $1`,
		in.GetProjectId(),
	).Scan(&total)

	return &pb_member.ListProjectMembersInvite_Response{
		Total:   total,
		Invites: invites,
	}, nil
}

// CreateProjectMemberInvite is ...
func (m *member) CreateProjectMemberInvite(ctx context.Context, in *pb_member.CreateProjectMemberInvite_Request) (*pb_member.CreateProjectMemberInvite_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	var invite string
	var inviteID string
	db.Conn.QueryRow(`SELECT 
			"id"
		FROM 
			"project_invite" 
		WHERE
			"project_invite"."email" = $1`,
		in.GetEmail(),
	).Scan(&inviteID)
	if inviteID != "" {
		return nil, errors.New("Email in use")
	}

	err := db.Conn.QueryRow(`INSERT 
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
		return nil, errors.New("ProjectMemberInvite failed")
	}

	return &pb_member.CreateProjectMemberInvite_Response{
		Invite: invite,
	}, nil
}

// DeleteProjectMemberInvite is ...
func (m *member) DeleteProjectMemberInvite(ctx context.Context, in *pb_member.DeleteProjectMemberInvite_Request) (*pb_member.DeleteProjectMemberInvite_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(internal.ErrNotFound)
	}

	_, err := db.Conn.Query(`DELETE 
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
		return nil, errors.New("DeleteProjectMemberInvite: failed")
	}

	return &pb_member.DeleteProjectMemberInvite_Response{}, nil
}

// ProjectMemberInviteActivate is ...
func (m *member) ProjectMemberInviteActivate(ctx context.Context, in *pb_member.ProjectMemberInviteActivate_Request) (*pb_member.ProjectMemberInviteActivate_Response, error) {
	var inviteID, userID, projectID, memberID, status string

	db.Conn.QueryRow(`SELECT
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
		return nil, errors.New("invite is invalid")
	}

	db.Conn.QueryRow(`SELECT
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
		return nil, errors.New("new user")
	}

	if userID != in.GetUserId() {
		return nil, errors.New("wrong user")
	}

	if status == "activated" {
		return nil, errors.New("invite is activated")
	}

	err := db.Conn.QueryRow(`INSERT 
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
		return nil, errors.New("member not added")
	}

	_, err = db.Conn.Exec(`UPDATE "project_invite" 
		SET 
			"status" = 'activated',
			"user_id" = $1
		WHERE 
			"invite" = $2`,
		in.GetUserId(),
		in.GetInvite(),
	)
	if err != nil {
		return nil, errors.New("failed to update invite information")
	}

	return &pb_member.ProjectMemberInviteActivate_Response{
		ProjectId: projectID,
	}, nil
}
