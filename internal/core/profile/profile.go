package profile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/core/notification"
	notificationpb "github.com/werbot/werbot/internal/core/notification/proto/notification"
	profilepb "github.com/werbot/werbot/internal/core/profile/proto/profile"
	"github.com/werbot/werbot/internal/core/token"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
	"github.com/werbot/werbot/pkg/utils/strutil"
)

// Profiles is lists all profiles on the system
func (h *Handler) Profiles(ctx context.Context, in *profilepb.Profiles_Request) (*profilepb.Profiles_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &profilepb.Profiles_Response{}

	// Total count for pagination
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT COUNT(*)
    FROM "profile"
  `).Scan(&response.Total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, trace.Error(err, log, nil)
	}
	if response.Total == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgProjectNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// List records
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	baseQuery := postgres.SQLGluing(`
    SELECT
      "profile"."id",
      "profile"."alias",
      "profile"."name",
      "profile"."surname",
      "profile"."email",
      "profile"."active",
      "profile"."confirmed",
      "profile"."role",
      "profile"."locked_at",
      "profile"."archived_at",
      "profile"."updated_at",
      "profile"."created_at",
      COUNT(DISTINCT "project"."id") AS "count_project",
      COUNT(DISTINCT "profile_public_key"."id") AS "count_keys",
      COUNT(DISTINCT "scheme"."id") AS "count_schemes"
    FROM "profile"
    LEFT JOIN "project" ON "project"."owner_id" = "profile"."id"
    LEFT JOIN "profile_public_key" ON "profile_public_key"."profile_id" = "profile"."id"
    LEFT JOIN "scheme" ON "scheme"."project_id" = "project"."id"
    GROUP BY
      "profile"."id",
      "profile"."alias",
      "profile"."name",
      "profile"."surname",
      "profile"."email",
      "profile"."active",
      "profile"."confirmed",
      "profile"."role",
      "profile"."locked_at",
      "profile"."archived_at",
      "profile"."updated_at",
      "profile"."created_at"
  `, sqlFooter)
	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
		profile := &profilepb.Profile_Response{}
		err = rows.Scan(
			&profile.ProfileId,
			&profile.Alias,
			&profile.Name,
			&profile.Surname,
			&profile.Email,
			&profile.Active,
			&profile.Confirmed,
			&profile.Role,
			&lockedAt,
			&archivedAt,
			&updatedAt,
			&createdAt,
			&profile.ProjectsCount,
			&profile.KeysCount,
			&profile.SchemesCount,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		protoutils.SetPgtypeTimestamps(response, map[string]pgtype.Timestamp{
			"locked_at":   lockedAt,
			"archived_at": archivedAt,
			"updated_at":  updatedAt,
			"created_at":  createdAt,
		})

		if !in.GetIsAdmin() {
			ghoster.Secrets(profile, true)
		}

		response.Profiles = append(response.Profiles, profile)
	}

	return response, nil
}

// Profile is displays profile information
func (h *Handler) Profile(ctx context.Context, in *profilepb.Profile_Request) (*profilepb.Profile_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
	response := &profilepb.Profile_Response{}
	response.ProfileId = in.GetProfileId()

	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "profile"."alias",
      "profile"."name",
      "profile"."surname",
      "profile"."email",
      "profile"."active",
      "profile"."confirmed",
      "profile"."role",
      "profile"."locked_at",
      "profile"."archived_at",
      "profile"."updated_at",
      "profile"."created_at",
      COUNT(DISTINCT "project"."id") AS "count_project",
      COUNT(DISTINCT "profile_public_key"."id") AS "count_keys",
      COUNT(DISTINCT "scheme"."id") AS "count_schemes"
    FROM "profile"
    LEFT JOIN "project" ON "project"."owner_id" = "profile"."id"
    LEFT JOIN "profile_public_key" ON "profile_public_key"."profile_id" = "profile"."id"
    LEFT JOIN "scheme" ON "scheme"."project_id" = "project"."id"
    WHERE "profile"."id" = $1
    GROUP BY
      "profile"."alias",
      "profile"."name",
      "profile"."surname",
      "profile"."email",
      "profile"."active",
      "profile"."confirmed",
      "profile"."role",
      "profile"."locked_at",
      "profile"."archived_at",
      "profile"."updated_at",
      "profile"."created_at"
  `, in.GetProfileId(),
	).Scan(
		&response.Alias,
		&response.Name,
		&response.Surname,
		&response.Email,
		&response.Active,
		&response.Confirmed,
		&response.Role,
		&lockedAt,
		&archivedAt,
		&updatedAt,
		&createdAt,
		&response.ProjectsCount,
		&response.KeysCount,
		&response.SchemesCount,
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	protoutils.SetPgtypeTimestamps(response, map[string]pgtype.Timestamp{
		"locked_at":   lockedAt,
		"archived_at": archivedAt,
		"updated_at":  updatedAt,
		"created_at":  createdAt,
	})

	if !in.GetIsAdmin() {
		ghoster.Secrets(response, true)
	}

	return response, nil
}

// AddProfile is adds a new profile
func (h *Handler) AddProfile(ctx context.Context, in *profilepb.AddProfile_Request) (*profilepb.AddProfile_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &profilepb.AddProfile_Response{}
	password, _ := crypto.HashPassword(in.Password, internal.GetInt("PASSWORD_HASH_COST", 13))
	err := h.DB.Conn.QueryRowContext(ctx, `
    INSERT INTO "profile" ("alias", "name", "surname", "email", "password", "active", "confirmed")
    SELECT $1, $2, $3, $4::varchar(64), $5, $6, $7
    WHERE NOT EXISTS (
      SELECT 1 FROM "profile" WHERE "email" = $4
    )
    RETURNING "id"
  `,
		in.GetAlias(),
		in.GetName(),
		in.GetSurname(),
		in.GetEmail(),
		password,
		in.GetActive(),
		in.GetConfirmed(),
	).Scan(&response.ProfileId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errGRPC := status.Error(codes.Canceled, trace.MsgFailedToAdd)
			return nil, trace.Error(errGRPC, log, nil)
		}
		return nil, trace.Error(err, log, "")
	}

	return response, nil
}

// UpdateProfile is updates profile data
func (h *Handler) UpdateProfile(ctx context.Context, in *profilepb.UpdateProfile_Request) (*profilepb.UpdateProfile_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	var column string
	var value any

	switch setting := in.GetSetting().(type) {
	case *profilepb.UpdateProfile_Request_Name:
		column = "name"
		value = in.GetName()
	case *profilepb.UpdateProfile_Request_Surname:
		column = "surname"
		value = in.GetSurname()

	case *profilepb.UpdateProfile_Request_Alias,
		*profilepb.UpdateProfile_Request_Email,
		*profilepb.UpdateProfile_Request_Confirmed,
		*profilepb.UpdateProfile_Request_Active,
		*profilepb.UpdateProfile_Request_Archived:
		if !in.GetIsAdmin() {
			errGRPC := status.Error(codes.InvalidArgument, "setting: exactly one field is required in oneof")
			return nil, trace.Error(errGRPC, log, nil)
		}

		switch setting.(type) {
		case *profilepb.UpdateProfile_Request_Alias:
			column = "alias"
			value = in.GetAlias()
		case *profilepb.UpdateProfile_Request_Email:
			column = "email"
			value = in.GetEmail()
		case *profilepb.UpdateProfile_Request_Confirmed:
			column = "confirmed"
			value = in.GetConfirmed()
		case *profilepb.UpdateProfile_Request_Active:
			column = "active"
			value = in.GetActive()
		case *profilepb.UpdateProfile_Request_Archived:
			column = "archived"
			value = in.GetArchived()
		}
	}

	query := fmt.Sprintf(`
    UPDATE "profile"
    SET "%s" = $1
    WHERE "id" = $2
  `, column)

	result, err := h.DB.Conn.ExecContext(ctx, query, value, in.GetProfileId())
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgUserNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &profilepb.UpdateProfile_Response{}, nil
}

// DeleteProfile is ...
func (h *Handler) DeleteProfile(ctx context.Context, in *profilepb.DeleteProfile_Request) (*profilepb.DeleteProfile_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	notification := notification.Handler{DB: h.DB, Worker: h.Worker}

	switch in.GetRequest().(type) {
	case *profilepb.DeleteProfile_Request_Password: // step 1
		// Get profile data
		var alias, passwordHash, email string
		err := h.DB.Conn.QueryRowContext(ctx, `
      SELECT "alias", "password", "email"
      FROM "profile"
      WHERE "id" = $1 AND "role" = 1
    `, in.GetProfileId()).Scan(
			&alias,
			&passwordHash,
			&email,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		if !crypto.CheckPasswordHash(in.GetPassword(), passwordHash) {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgPasswordIsNotValid)
			return nil, trace.Error(errGRPC, log, nil)
		}

		// Get or create token via token package
		tokenHandler := token.Handler{DB: h.DB, Worker: h.Worker}
		tokenID, isNew, err := tokenHandler.GetOrCreateProfileToken(ctx, in.GetProfileId(), tokenenum.Action_delete, func(ctx context.Context, profileID string) (string, error) {
			tokenResp, err := tokenHandler.AddTokenProfileDelete(ctx, &tokenmessage.AddTokenProfileDelete_Request{
				ProfileId: profileID,
			})
			if err != nil {
				return "", err
			}
			return tokenResp.GetToken(), nil
		})
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		// send email with token link
		if _, err := notification.SendMail(ctx, &notificationpb.SendMail_Request{
			Email:    email,
			Subject:  "profile deletion confirmation",
			Template: notificationpb.MailTemplate_account_deletion_confirmation,
			Data: map[string]string{
				"Login":     alias,
				"Link":      fmt.Sprintf("%s/profile/setting/destroy/%s", internal.GetString("APP_DSN", "http://localhost:5173"), tokenID),
				"FirstSend": strconv.FormatBool(isNew),
			},
		}); err != nil {
			return nil, trace.Error(err, log, nil)
		}

	case *profilepb.DeleteProfile_Request_Token: // step 2
		// Check token via token package
		tokenHandler := token.Handler{DB: h.DB, Worker: h.Worker}
		tokenData, err := tokenHandler.Token(ctx, &tokenmessage.Token_Request{
			IsAdmin: false,
			Token:   in.GetToken(),
		})
		if err != nil {
			return nil, trace.Error(status.Error(codes.InvalidArgument, trace.MsgTokenIsInvalid), log, nil)
		}

		// Verify token is for delete profile and has correct status
		if tokenData.GetAction() != tokenenum.Action_delete || tokenData.GetStatus() != tokenenum.Status_sent {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgTokenIsInvalid)
			return nil, trace.Error(errGRPC, log, nil)
		}

		// Verify token belongs to specified profile
		if tokenData.GetProfileId() != in.GetProfileId() {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgTokenIsInvalid)
			return nil, trace.Error(errGRPC, log, nil)
		}

		// Get profile data
		var alias, email string
		err = h.DB.Conn.QueryRowContext(ctx, `
      SELECT "alias", "email"
      FROM "profile"
      WHERE "id" = $1 AND "role" = 1
    `, in.GetProfileId()).Scan(&alias, &email)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		tx, err := h.DB.Conn.BeginTx(ctx, nil)
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgTransactionCreateError)
		}
		defer tx.Rollback()

		// disable profile
		archivedAt := time.Now().AddDate(0, 1, 0)

		_, err = tx.ExecContext(ctx, `
      UPDATE "profile"
      SET
        "active" = false,
        "locked_at" = NOW(),
        "archived_at" = $2
      WHERE "id" = $1
    `,
			in.GetProfileId(),
			archivedAt,
		)
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
		}

		// Update token status to used via token package
		// Use direct SQL within transaction, as UpdateProfileToken uses its own transaction
		_, err = tx.ExecContext(ctx, `
      UPDATE "token"
      SET "status" = $1
      WHERE "id" = $2
    `, tokenenum.Status_used, in.GetToken())
		if err != nil {
			return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
		}

		if err := tx.Commit(); err != nil {
			return nil, trace.Error(err, log, trace.MsgTransactionCommitError)
		}

		// send delete token to email
		notification.SendMail(ctx, &notificationpb.SendMail_Request{
			Email:    email,
			Subject:  "profile deleted",
			Template: notificationpb.MailTemplate_account_deletion_info,
			Data: map[string]string{
				"Login": alias,
			},
		})
	}

	return &profilepb.DeleteProfile_Response{}, nil
}

// UpdatePassword is ...
func (h *Handler) UpdatePassword(ctx context.Context, in *profilepb.UpdatePassword_Request) (*profilepb.UpdatePassword_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	// Check for a validity of the old password
	var currentPassword string
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT "password"
    FROM "profile"
    WHERE "id" = $1
  `, in.GetProfileId()).Scan(&currentPassword)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	if !crypto.CheckPasswordHash(in.GetOldPassword(), currentPassword) {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgPasswordIsNotValid)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// We change the old password for a new
	newPassword, err := crypto.HashPassword(in.GetNewPassword(), internal.GetInt("PASSWORD_HASH_COST", 13))
	if err != nil {
		errGRPC := status.Error(codes.InvalidArgument, trace.MsgPasswordIsNotValid)
		return nil, trace.Error(errGRPC, log, nil)
	}

	result, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "profile"
    SET "password" = $1
    WHERE "id" = $2
  `,
		newPassword,
		in.GetProfileId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgUserNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return &profilepb.UpdatePassword_Response{}, nil
}

// TODO Check bu invite and Enable check in Firewall
// ProfileIDByLogin is a function that takes a context and an AccountIDByLogin_Request as input,
// and returns an AccountIDByLogin_Response and an error as output.
func (h *Handler) ProfileIDByLogin(ctx context.Context, in *profilepb.ProfileIDByLogin_Request) (*profilepb.ProfileIDByLogin_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &profilepb.ProfileIDByLogin_Response{}
	nameArray := strutil.SplitNTrimmed(in.GetLogin(), "_", 3)

	stmt, err := h.DB.Conn.PrepareContext(ctx, `
    SELECT "profile"."id"
    FROM
      "profile"
      JOIN "profile_public_key" ON "profile".i"d = "profile_public_key"."profile_id"
    WHERE
      "profile"."login" = $1
      AND "profile_public_key"."fingerprint" = $2
  `)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer stmt.Close()

	err = stmt.QueryRowContext(ctx, nameArray[0], in.GetFingerprint()).Scan(&response.ProfileId)
	if err != nil {
		if err == sql.ErrNoRows {
			errGRPC := status.Error(codes.NotFound, trace.MsgAccountNotFound)
			return nil, trace.Error(errGRPC, log, nil)
		}

		return nil, trace.Error(err, log, nil)
	}

	return response, nil
}

// ProfileByEmail is ...
func (h *Handler) ProfileByEmail(ctx context.Context, in *profilepb.ProfileByEmail_Request) (*profilepb.Profile_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
	response := &profilepb.Profile_Response{}
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "id",
      "alias",
      "name",
      "surname",
      "email",
      "active",
      "confirmed",
      "role",
      "locked_at",
      "archived_at",
      "updated_at",
      "created_at"
    FROM "profile"
    WHERE "email" = $1
  `, in.GetEmail()).Scan(
		&response.ProfileId,
		&response.Alias,
		&response.Name,
		&response.Surname,
		&response.Email,
		&response.Active,
		&response.Confirmed,
		&response.Role,
		&lockedAt,
		&archivedAt,
		&updatedAt,
		&createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			errGRPC := status.Error(codes.NotFound, trace.MsgAccountNotFound)
			return nil, trace.Error(errGRPC, log, nil)
		}
		return nil, trace.Error(err, log, nil)
	}

	protoutils.SetPgtypeTimestamps(response, map[string]pgtype.Timestamp{
		"locked_at":   lockedAt,
		"archived_at": archivedAt,
		"updated_at":  updatedAt,
		"created_at":  createdAt,
	})

	return response, nil
}

// UpdateStatus is a method implemented by Handler struct which accepts
// a context and an UpdateStatus_Request object and returns an UpdateStatus_Response object and an error
func (h *Handler) UpdateStatus(ctx context.Context, in *profilepb.UpdateStatus_Request) (*profilepb.UpdateStatus_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &profilepb.UpdateStatus_Response{}

	online := false
	if in.GetStatus() == 1 {
		online = true
	}

	res, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "scheme_member"
    SET "online" = $2
    WHERE "id" = $1
  `,
		in.GetAccountId(),
		online,
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgAccountNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return response, nil
}
