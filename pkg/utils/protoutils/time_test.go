package protoutils

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/werbot/werbot/pkg/utils/maputil"
	protoutils "github.com/werbot/werbot/pkg/utils/protoutils/test"
)

func Test_SetPgtypeTimestamps(t *testing.T) {
	currentTime := time.Now()
	timeStamps := []pgtype.Timestamp{
		{Time: currentTime, Valid: true},                     // lockedAt
		{Time: currentTime.Add(-1 * time.Hour), Valid: true}, // archivedAt
		{Time: currentTime.Add(-2 * time.Hour), Valid: true}, // updatedAt
		{Time: currentTime.Add(-3 * time.Hour), Valid: true}, // createdAt
	}

	cases := []struct {
		debug   bool
		name    string
		options *SetPgtypeTimestampsOptions
		stamps  map[string]pgtype.Timestamp
		resp    map[string]any
	}{
		{
			// debug: true,
			name: "default options",
			stamps: map[string]pgtype.Timestamp{
				"locked_at": timeStamps[0],
				// "archived_at": timeStamps[1],
				"updated_at": timeStamps[2],
				"created_at": timeStamps[3],
				"timestamp":  timeStamps[0],
			},
			resp: map[string]any{
				"locked_at":   "*",
				"archived_at": nil,
				"updated_at":  "*",
				"created_at":  "*",
				"timestamp":   nil,
			},
		},
		{
			// debug: true,
			name:   "default options with empty data",
			stamps: map[string]pgtype.Timestamp{},
			resp: map[string]any{
				"locked_at":   nil,
				"archived_at": nil,
				"updated_at":  nil,
				"created_at":  nil,
			},
		},
		{
			// debug: true,
			name: "custom options",
			options: &SetPgtypeTimestampsOptions{
				FieldNames: []protoreflect.Name{"timestamp"},
			},
			stamps: map[string]pgtype.Timestamp{
				"locked_at":   timeStamps[0],
				"archived_at": timeStamps[1],
				"updated_at":  timeStamps[2],
				"created_at":  timeStamps[3],
				"timestamp":   timeStamps[0],
			},
			resp: map[string]any{
				"locked_at":   nil,
				"archived_at": nil,
				"updated_at":  nil,
				"created_at":  nil,
				"timestamp":   "*",
			},
		},
		{
			// debug: true,
			name: "custom options with empty data",
			options: &SetPgtypeTimestampsOptions{
				FieldNames: []protoreflect.Name{"timestamp"},
			},
			stamps: map[string]pgtype.Timestamp{
				"locked_at": timeStamps[0],
			},
			resp: map[string]any{
				"locked_at": nil,
				"timestamp": nil,
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			testMessage := &protoutils.MessageTest{}

			if tt.options != nil {
				tt.options.SetPgtypeTimestamps(testMessage, tt.stamps)
			} else {
				SetPgtypeTimestamps(testMessage, tt.stamps)
			}

			testResponse, _ := protojson.MarshalOptions{
				UseEnumNumbers: true,
				UseProtoNames:  true,
				Multiline:      true,
			}.Marshal(testMessage)

			var protoResp map[string]any
			json.Unmarshal(testResponse, &protoResp)

			if tt.debug {
				t.Logf("\nDebug data: %s", testResponse)
			}

			for key, want := range tt.resp {
				val, _ := maputil.GetByPath(key, protoResp)
				if want == "*" {
					assert.NotEmpty(t, val, "Key [%s] was incorrect", key)
				} else {
					assert.Equal(t, want, val, "Key [%s] was incorrect", key)
				}
			}
		})
	}
}
