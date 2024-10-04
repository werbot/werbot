package ghoster

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	ghosterpb "github.com/werbot/werbot/pkg/utils/protoutils/ghoster/proto"
)

// Secrets  is ...
func Secrets(msg proto.Message, nuller bool) {
	secretReplacementMask := "***"
	msgReflect := msg.ProtoReflect()

	handleField := func(fd protoreflect.FieldDescriptor, v protoreflect.Value) {
		if options, ok := fd.Options().(*descriptorpb.FieldOptions); ok {
			if ext, ok := proto.GetExtension(options, ghosterpb.E_Secret).(bool); ok && ext {
				if !nuller && fd.Kind() == protoreflect.StringKind {
					msgReflect.Set(fd, protoreflect.ValueOfString(secretReplacementMask))
				} else {
					msgReflect.Clear(fd)
				}
			}
		}
	}

	msgReflect.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch fd.Kind() {
		case protoreflect.MessageKind:
			switch fd.Message().FullName() {
			case "google.protobuf.Timestamp", "google.protobuf.Duration",
				"google.protobuf.DoubleValue", "google.protobuf.FloatValue",
				"google.protobuf.Int64Value", "google.protobuf.UInt64Value",
				"google.protobuf.Int32Value", "google.protobuf.UInt32Value",
				"google.protobuf.BoolValue", "google.protobuf.BytesValue":
				handleField(fd, v)
			case "google.protobuf.StringValue":
				if !nuller {
					msgReflect.Set(fd, protoreflect.ValueOfMessage(wrapperspb.String(secretReplacementMask).ProtoReflect()))
				} else {
					msgReflect.Clear(fd)
				}
			default:
				Secrets(v.Message().Interface(), nuller)
			}
		case protoreflect.StringKind, protoreflect.Int32Kind, protoreflect.Int64Kind,
			protoreflect.Uint32Kind, protoreflect.Uint64Kind,
			protoreflect.FloatKind, protoreflect.DoubleKind,
			protoreflect.BoolKind, protoreflect.BytesKind:
			handleField(fd, v)
		}
		return true
	})
}
