package grpc

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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
