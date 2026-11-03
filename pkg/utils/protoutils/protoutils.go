package protoutils

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/werbot/werbot/pkg/utils/errutil"
)

var errFailedValidator = errors.New("failed to initialize validator")

// ValidateRequest validates a protobuf message using protovalidate.
func ValidateRequest(request proto.Message) error {
	v, err := protovalidate.New()
	if err != nil {
		return errFailedValidator
	}

	if err := v.Validate(request); err != nil {
		errorList := make(errutil.ErrorMap)
		validErr := err.(*protovalidate.ValidationError)
		for _, violation := range validErr.Violations {
			// Get field path from FieldDescriptor. A violation raised on the
			// message itself (oneof, message-level CEL) has no field descriptor.
			fieldPath := ""
			if violation.FieldDescriptor != nil {
				fieldPath = string(violation.FieldDescriptor.Name())
			}
			if violation.Proto != nil {
				// Try to get field path from proto if available
				if field := violation.Proto.GetField(); field != nil && len(field.GetElements()) > 0 {
					var pathParts []string
					for _, elem := range field.GetElements() {
						if elem.GetFieldName() != "" {
							pathParts = append(pathParts, elem.GetFieldName())
						}
						if elem.GetSubscript() != nil && len(pathParts) > 0 {
							pathParts[len(pathParts)-1] += fmt.Sprintf("[%d]", elem.GetIndex())
						}
					}
					if len(pathParts) > 0 {
						fieldPath = pathParts[0]
						for _, part := range pathParts[1:] {
							fieldPath += "." + part
						}
					}
				}
				message := violation.Proto.GetMessage()
				if message == "" {
					message = violation.String()
				}
				// protovalidate v1.4 reworded the builtin constraint messages
				// ("value must be at least 3 characters" instead of "value length
				// must be at least 3 characters") and dropped the "value " prefix
				// from some. Custom CEL messages and oneof/required-oneof
				// violations are already full sentences and are left untouched.
				// The API contract and its tests still report the pre-1.4 wording
				// for builtin field rules.
				ruleName := ""
				if violation.RuleDescriptor != nil {
					ruleName = string(violation.RuleDescriptor.Name())
				}
				isOneof := strings.Contains(message, "exactly one field is required in oneof")
				if ruleName != "cel" && ruleName != "required" && !isOneof {
					if !strings.HasPrefix(message, "value ") {
						message = "value " + message
					}
					if strings.HasPrefix(message, "value must be at least ") ||
						strings.HasPrefix(message, "value must be at most ") {
						message = "value length" + strings.TrimPrefix(message, "value")
					}
				}
				errorList[fieldPath] = message
			} else {
				// Fallback to String() if Proto is nil
				errorList[fieldPath] = violation.String()
			}
		}

		return errorList
	}

	return nil
}

// ConvertProtoMessageToMap converts a proto.Message to a map[string]any
func ConvertProtoMessageToMap(msg proto.Message) (map[string]any, error) {
	// Marshal the proto message to JSON
	marshaler := protojson.MarshalOptions{
		UseEnumNumbers: true,
		UseProtoNames:  true,
	}

	jsonData, err := marshaler.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proto message to JSON: %w", err)
	}

	// Unmarshal the JSON data into a map
	var resultMap map[string]any
	if err := json.Unmarshal(jsonData, &resultMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}

	return resultMap, nil
}
