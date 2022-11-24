package grpc

import (
	"context"
	"errors"

	"github.com/jackc/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/werbot/werbot/internal/message"

	pb_member "github.com/werbot/werbot/internal/grpc/proto/member"
	pb_user "github.com/werbot/werbot/internal/grpc/proto/user"
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
		return nil, errors.New(message.ErrNotFound)
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
		return nil, errors.New(message.ErrNotFound)
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
		return nil, errors.New(message.ErrNotFound)
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
		return nil, errors.New(message.ErrNotFound)
	}

	var created pgtype.Timestamp
	var ownerName, projectName, userName string

	_, err := db.Conn.Exec(`UPDATE "project_member" 
		SET "role" = $1, 
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
			"project_member"."id" = $1`, in.GetMemberId()).
		Scan(
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
		return nil, errors.New(message.ErrNotFound)
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
	ct, err := db.Conn.Exec(`	UPDATE
			"project_member"
		SET
			"active" = $1
		FROM
			"project"
		WHERE
			"project_member"."id" = $2 AND 
			"project"."owner_id"  = $3 AND
			"project_member"."project_id" = "project"."id"`, in.GetStatus(), in.GetMemberId(), in.GetOwnerId())
	if err != nil {
		return &pb_member.UpdateProjectMemberStatus_Response{}, err
	}
	if rows, _ := ct.RowsAffected(); rows != 1 {
		return &pb_member.UpdateProjectMemberStatus_Response{}, errors.New(message.ErrNotFound)
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

	rows, err := db.Conn.Query(`SELECT "id" AS "member_id", "name" AS "member_name", "email" FROM "user" WHERE "user"."name" LIKE '$1%'  ORDER BY "name" ASC LIMIT 15 OFFSET 0`, in.Name)
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
		return nil, errors.New(message.ErrNotFound)
	}

	users := []*pb_member.GetUsersWithoutProject_Response_User{}
	rows, err := db.Conn.Queryx(`SELECT
			"id",
			"name",
			"email"
		FROM
			"user"
		WHERE
			"id" NOT IN(
				SELECT
					"user_id" FROM "project_member"
				WHERE
					"project_id" = $1)
		AND LOWER("name") LIKE LOWER($2 || '%')
		ORDER BY
			"name" ASC
		LIMIT 15 OFFSET 0`, in.GetProjectId(), in.GetName())
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
		return nil, errors.New(message.ErrNotFound)
	}

	sqlFooter := db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())

	members := []*pb_member.GetServerMember_Response{}
	rows, err := db.Conn.Query(`SELECT
			"user"."id"
			"user"."name",
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
			AND "server_member"."server_id" = $2`+sqlFooter, in.GetProjectId(), in.GetServerId())
	if err != nil {
		return nil, errors.New("GetServerMembers: query parameters failed")
	}

	for rows.Next() {
		member := pb_member.GetServerMember_Response{}
		var lastActivity pgtype.Timestamp

		err = rows.Scan(
			&member.UserId,
			&member.UserName,
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
			AND "server_member"."server_id" = $2`, in.GetProjectId(), in.GetServerId()).Scan(&total)

	return &pb_member.ListServerMembers_Response{
		Total:   total,
		Members: members,
	}, nil
}

// GetServerMember is ...
func (m *member) GetServerMember(ctx context.Context, in *pb_member.GetServerMember_Request) (*pb_member.GetServerMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(message.ErrNotFound)
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
			"server_member"."id" = $1`, in.GetMemberId()).
		Scan(
			&member.UserId,
			&member.UserName,
			&member.Active,
			&member.Online,
			&lastActivity,
		)
	if err != nil {
		return nil, errors.New(message.ErrNotFound)
	}

	member.MemberId = in.GetMemberId()
	member.LastActivity = timestamppb.New(lastActivity.Time)

	return &member, nil
}

// CreateServerMember is ...
func (m *member) CreateServerMember(ctx context.Context, in *pb_member.CreateServerMember_Request) (*pb_member.CreateServerMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(message.ErrNotFound)
	}

	var accountID int32
	err := db.Conn.QueryRowx(`INSERT INTO "server_member" (
		"server_id",
		"member_id",
		"active"
	)
	VALUES
		($1, $2, $3)
	RETURNING "id"`,
		in.GetServerId(),
		in.GetMemberId(),
		in.GetActive(),
	).Scan(&accountID)
	if err != nil {
		return nil, errors.New("CreateMember failed")
	}

	return &pb_member.CreateServerMember_Response{
		AccountId: accountID,
	}, nil
}

// UpdateServerMember is ...
func (m *member) UpdateServerMember(ctx context.Context, in *pb_member.UpdateServerMember_Request) (*pb_member.UpdateServerMember_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(message.ErrNotFound)
	}

	_, err := db.Conn.Exec(`UPDATE "server_member" 
		SET 
			"active" = $1 
		WHERE 
			"id" = $2 
			AND "server_id" = $3`,
		in.GetActive(),
		in.GetAccountId(),
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
		return nil, errors.New(message.ErrNotFound)
	}

	_, err := db.Conn.Exec(`DELETE FROM "server_member" WHERE "id" = $1 AND "server_id" = $2`, in.GetAccountId(), in.GetServerId())
	if err != nil {
		return &pb_member.DeleteServerMember_Response{}, errors.New("DeleteMember failed")
	}

	return &pb_member.DeleteServerMember_Response{}, nil
}

// UpdateServerMemberStatus is ...
func (m *member) UpdateServerMemberStatus(ctx context.Context, in *pb_member.UpdateServerMemberStatus_Request) (*pb_member.UpdateServerMemberStatus_Response, error) {
	ct, err := db.Conn.Exec(`UPDATE
			"server_member"
		SET
			"active" = $1
		FROM
			"project"
		WHERE
			"project_member"."id" = $2 AND 
			"project"."owner_id"  = $3 AND
			"server_member"."id" = $4 AND
			"project_member"."project_id" = "project"."id"`, in.GetStatus(), in.GetMemberId(), in.GetOwnerId(), in.GetServerId())
	if err != nil {
		return &pb_member.UpdateServerMemberStatus_Response{}, err
	}
	if rows, _ := ct.RowsAffected(); rows != 1 {
		return &pb_member.UpdateServerMemberStatus_Response{}, errors.New(message.ErrNotFound)
	}

	return &pb_member.UpdateServerMemberStatus_Response{}, nil
}

// GetMemberWithoutServer
func (m *member) GetMemberWithoutServer(ctx context.Context, in *pb_member.GetMemberWithoutServer_Request) (*pb_member.GetMemberWithoutServer_Response, error) {
	if !checkUserIDAndProjectID(in.GetProjectId(), in.GetOwnerId()) {
		return nil, errors.New(message.ErrNotFound)
	}

	members := []*pb_member.GetMemberWithoutServer_Response_Member{}

	rows, err := db.Conn.Queryx(`SELECT
			"project_member"."id",
			"user"."name",
			"user"."email"
		FROM
			"project_member"
			INNER JOIN "user" ON "project_member"."user_id" = "user"."id"
		WHERE
			"project_member"."id" NOT IN(
				SELECT
					"member_id" FROM "server_member"
				WHERE
					"server_id" = $1)
			AND "project_member"."project_id" = $2
			AND LOWER("user"."name") LIKE LOWER($3 || '%')
		ORDER BY
			"user"."name" ASC
		LIMIT 15 OFFSET 0`, in.GetServerId(), in.GetProjectId(), in.GetName())

	if err != nil {
		return nil, errors.New("Action GetUsersWithoutServer query parameters failed")
	}

	for rows.Next() {
		member := pb_member.GetMemberWithoutServer_Response_Member{}

		err = rows.Scan(
			&member.MemberId,
			&member.Name,
			&member.Email,
		)
		if err != nil {
			return nil, errors.New("GetUsersWithoutServer scan failed")
		}

		members = append(members, &member)
	}
	defer rows.Close()

	return &pb_member.GetMemberWithoutServer_Response{
		Members: members,
	}, nil
}

// CreateMemberInvite is ...
func (m *member) CreateMemberInvite(ctx context.Context, in *pb_member.CreateMemberInvite_Request) (*pb_member.CreateMemberInvite_Response, error) {
	return &pb_member.CreateMemberInvite_Response{}, nil
}
