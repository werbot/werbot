package protoutils

import (
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func SetPgtypeTimestamps(msg protoreflect.ProtoMessage, timestamps map[string]pgtype.Timestamp) {
	SetPgtypeTimestampsOptions{
		FieldNames: []protoreflect.Name{"locked_at", "archived_at", "updated_at", "created_at"},
	}.SetPgtypeTimestamps(msg, timestamps)
}

type SetPgtypeTimestampsOptions struct {
	FieldNames []protoreflect.Name
}

func (o SetPgtypeTimestampsOptions) SetPgtypeTimestamps(msg protoreflect.ProtoMessage, timestamps map[string]pgtype.Timestamp) {
	msgReflect := msg.ProtoReflect()
	msgDescriptor := msgReflect.Descriptor()
	fields := msgDescriptor.Fields()

	for _, fieldName := range o.FieldNames {
		if ts, exists := timestamps[string(fieldName)]; exists && ts.Valid {
			if field := fields.ByName(fieldName); field != nil {
				tsProto := timestamppb.New(ts.Time)
				msgReflect.Set(field, protoreflect.ValueOfMessage(tsProto.ProtoReflect()))
			}
		}
	}
}
