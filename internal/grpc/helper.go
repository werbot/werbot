package grpc

import (
	"context"
	"errors"
	"strings"

	"github.com/bufbuild/protovalidate-go"
	"github.com/werbot/werbot/pkg/errutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// ensureValidToken ensures that a valid token is present in the incoming request metadata.
// If a valid token is not present, return an error stating the reason.
func ensureValidToken(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "Missing metadata")
	}

	authHeader, ok := md["authorization"]
	if !ok || len(authHeader) == 0 {
		return nil, status.Error(codes.Unauthenticated, "Missing authorization token")
	}

	token := strings.TrimPrefix(authHeader[0], "Bearer ")
	if token != service.token {
		return nil, status.Error(codes.Unauthenticated, "Invalid authorization token")
	}

	return handler(ctx, req)
}

func ValidateRequest(request proto.Message) error {
	v, err := protovalidate.New()
	if err != nil {
		return errors.New("failed to initialize validator")
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
