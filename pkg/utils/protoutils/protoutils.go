package protoutils

import (
	"encoding/json"
	"errors"
	"fmt"

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
			// Get field path from FieldDescriptor
			fieldPath := string(violation.FieldDescriptor.Name())
			if violation.Proto != nil {
				// Try to get field path from proto if available
				if field := violation.Proto.GetField(); field != nil && len(field.GetElements()) > 0 {
					var pathParts []string
					for _, elem := range field.GetElements() {
						if elem.GetFieldName() != "" {
							pathParts = append(pathParts, elem.GetFieldName())
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
