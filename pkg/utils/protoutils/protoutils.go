package protoutils

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bufbuild/protovalidate-go"
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
		validErr := err.(*protovalidate.ValidationError).ToProto().GetViolations()
		for _, errTmp := range validErr {
			errorList[errTmp.GetFieldPath()] = errTmp.GetMessage()
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
