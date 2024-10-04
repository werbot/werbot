package ghoster

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/werbot/werbot/pkg/utils/maputil"
	ghosterpb "github.com/werbot/werbot/pkg/utils/protoutils/ghoster/test"
)

func Test_Secrets(t *testing.T) {
	testMessage := &ghosterpb.MessageTest{
		ProtoString:     "string",
		ProtoInt32:      -2147483647,
		ProtoInt64:      -9223372036854775807,
		ProtoUint32:     4294967295,
		ProtoUint64:     18446744073709551615,
		ProtoFloat:      0.123456789121212,
		ProtoDouble:     0.123456789121212121212,
		ProtoBool:       true,
		ProtoBytes:      []byte("bytes"),
		GoogleTimestamp: timestamppb.New(time.Now()),
		GoogleDuration:  durationpb.New(time.Hour),
		GoogleDouble:    wrapperspb.Double(0.123456789121212121212),
		GoogleFloat:     wrapperspb.Float(0.123456789121212),
		GoogleInt64:     wrapperspb.Int64(-9223372036854775807),
		GoogleUint64:    wrapperspb.UInt64(18446744073709551615),
		GoogleInt32:     wrapperspb.Int32(-2147483647),
		GoogleUint32:    wrapperspb.UInt32(4294967295),
		GoogleBool:      wrapperspb.Bool(true),
		GoogleString:    wrapperspb.String("string"),
		GoogleBytes:     wrapperspb.Bytes([]byte("bytes")),
	}

	cases := []struct {
		name   string
		nuller bool
		resp   map[string]any
	}{
		{
			name:   "nulled is disabled",
			nuller: false,
			resp: map[string]any{
				"proto_string":     "***",
				"proto_int32":      nil,
				"proto_int64":      nil,
				"proto_uint32":     nil,
				"proto_uint64":     nil,
				"proto_float":      nil,
				"proto_double":     nil,
				"proto_bool":       nil,
				"proto_bytes":      nil,
				"google_timestamp": nil,
				"google_duration":  nil,
				"google_double":    nil,
				"google_float":     nil,
				"google_int64":     nil,
				"google_uint64":    nil,
				"google_int32":     nil,
				"google_uint32":    nil,
				"google_bool":      nil,
				"google_string":    "***",
				"google_bytes":     nil,
				/*
					"proto_string":     "***",
					"proto_int32":      float64(-2.147483647e+09),
					"proto_int64":      "-9223372036854775807",
					"proto_uint32":     float64(4.294967295e+09),
					"proto_uint64":     "18446744073709551615",
					"proto_float":      0.12345679,
					"proto_double":     0.12345678912121212,
					"proto_bool":       true,
					"proto_bytes":      "Ynl0ZXM=",
					"google_timestamp": "*",
					"google_duration":  "3600s",
					"google_double":    0.12345678912121212,
					"google_float":     0.12345679,
					"google_int64":     "-9223372036854775807",
					"google_uint64":    "18446744073709551615",
					"google_int32":     float64(-2.147483647e+09),
					"google_uint32":    float64(4.294967295e+09),
					"google_bool":      true,
					"google_string":    "***",
					"google_bytes":     "Ynl0ZXM=",
				*/
			},
		},
		{
			name:   "nulled is enabled",
			nuller: true,
			resp: map[string]any{
				"proto_string":     nil,
				"proto_int32":      nil,
				"proto_int64":      nil,
				"proto_uint32":     nil,
				"proto_uint64":     nil,
				"proto_float":      nil,
				"proto_double":     nil,
				"proto_bool":       nil,
				"proto_bytes":      nil,
				"google_timestamp": nil,
				"google_duration":  nil,
				"google_double":    nil,
				"google_float":     nil,
				"google_int64":     nil,
				"google_uint64":    nil,
				"google_int32":     nil,
				"google_uint32":    nil,
				"google_bool":      nil,
				"google_string":    nil,
				"google_bytes":     nil,
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			Secrets(testMessage, tt.nuller)

			testResponse, _ := protojson.MarshalOptions{
				UseEnumNumbers: true,
				UseProtoNames:  true,
				Multiline:      true,
			}.Marshal(testMessage)

			var protoResp map[string]any
			json.Unmarshal(testResponse, &protoResp)

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
