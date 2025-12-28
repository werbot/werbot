package token_test

import (
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/stretchr/testify/assert"
	"github.com/werbot/werbot/internal/core/token"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	"github.com/werbot/werbot/internal/trace"
)

func Test_ValidateTokenStatusAndAction(t *testing.T) {
	testTable := []struct {
		name           string
		tokenStatus    tokenenum.Status
		action         tokenenum.Action
		expectedAction tokenenum.Action
		expectedStatus tokenenum.Status
		wantErr        bool
		wantCode       codes.Code
	}{
		{
			name:           "valid_status_and_action",
			tokenStatus:    tokenenum.Status_sent,
			action:         tokenenum.Action_reset,
			expectedAction: tokenenum.Action_reset,
			expectedStatus: tokenenum.Status_sent,
			wantErr:        false,
		},
		{
			name:           "invalid_action",
			tokenStatus:    tokenenum.Status_sent,
			action:         tokenenum.Action_register,
			expectedAction: tokenenum.Action_reset,
			expectedStatus: tokenenum.Status_sent,
			wantErr:        true,
			wantCode:       codes.InvalidArgument,
		},
		{
			name:           "action_mismatch_returns_error",
			tokenStatus:    tokenenum.Status_sent,
			action:         tokenenum.Action_delete,
			expectedAction: tokenenum.Action_reset,
			expectedStatus: tokenenum.Status_sent,
			wantErr:        true,
			wantCode:       codes.InvalidArgument,
		},
		{
			name:           "invalid_status",
			tokenStatus:    tokenenum.Status_used,
			action:         tokenenum.Action_reset,
			expectedAction: tokenenum.Action_reset,
			expectedStatus: tokenenum.Status_sent,
			wantErr:        true,
			wantCode:       codes.InvalidArgument,
		},
		{
			name:           "both_invalid",
			tokenStatus:    tokenenum.Status_used,
			action:         tokenenum.Action_register,
			expectedAction: tokenenum.Action_reset,
			expectedStatus: tokenenum.Status_sent,
			wantErr:        true,
			wantCode:       codes.InvalidArgument,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			err := token.ValidateTokenStatusAndAction(
				tt.tokenStatus,
				tt.action,
				tt.expectedAction,
				tt.expectedStatus,
			)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantCode > 0 {
					dataError := trace.ParseError(err)
					assert.Equal(t, tt.wantCode.String(), dataError.Code.String())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_ValidateTokenForUpdate(t *testing.T) {
	testTable := []struct {
		name          string
		isAdmin       bool
		currentStatus tokenenum.Status
		newStatus     tokenenum.Status
		wantErr       bool
		wantCode      codes.Code
	}{
		{
			name:          "admin_can_set_any_status",
			isAdmin:       true,
			currentStatus: tokenenum.Status_sent,
			newStatus:     tokenenum.Status_deleted,
			wantErr:       false,
		},
		{
			name:          "admin_can_update_done_status",
			isAdmin:       true,
			currentStatus: tokenenum.Status_done,
			newStatus:     tokenenum.Status_used,
			wantErr:       false,
		},
		{
			name:          "user_can_set_valid_status",
			isAdmin:       false,
			currentStatus: tokenenum.Status_sent,
			newStatus:     tokenenum.Status_used,
			wantErr:       false,
		},
		{
			name:          "user_cannot_set_deleted_status",
			isAdmin:       false,
			currentStatus: tokenenum.Status_sent,
			newStatus:     tokenenum.Status_deleted,
			wantErr:       true,
			wantCode:      codes.NotFound,
		},
		{
			name:          "user_cannot_set_archived_status",
			isAdmin:       false,
			currentStatus: tokenenum.Status_sent,
			newStatus:     tokenenum.Status_archived,
			wantErr:       true,
			wantCode:      codes.NotFound,
		},
		{
			name:          "user_cannot_set_unspecified_status",
			isAdmin:       false,
			currentStatus: tokenenum.Status_sent,
			newStatus:     tokenenum.Status_status_unspecified,
			wantErr:       true,
			wantCode:      codes.NotFound,
		},
		{
			name:          "user_cannot_update_done_status",
			isAdmin:       false,
			currentStatus: tokenenum.Status_done,
			newStatus:     tokenenum.Status_used,
			wantErr:       true,
			wantCode:      codes.PermissionDenied,
		},
		{
			name:          "user_can_update_sent_to_done",
			isAdmin:       false,
			currentStatus: tokenenum.Status_sent,
			newStatus:     tokenenum.Status_done,
			wantErr:       false,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			err := token.ValidateTokenForUpdate(
				tt.isAdmin,
				tt.currentStatus,
				tt.newStatus,
			)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantCode > 0 {
					dataError := trace.ParseError(err)
					assert.Equal(t, tt.wantCode.String(), dataError.Code.String())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
