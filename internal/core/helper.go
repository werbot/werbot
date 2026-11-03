package grpc

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/pkg/logger"
)

var log = logger.New()

// unaryRecoveryInterceptor converts a panic in a handler into an Internal
// error so one broken RPC cannot take down the whole server.
func unaryRecoveryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error(fmt.Errorf("panic recovered: %v", recovered)).
				Str("method", info.FullMethod).
				Bytes("stack", debug.Stack()).
				Send()

			resp = nil
			err = status.Error(codes.Internal, "Internal server error")
		}
	}()

	return handler(ctx, req)
}

// ensureValidToken ensures that a valid token is present in the incoming request metadata.
// If a valid token is not present, return an error stating the reason.
func ensureValidToken(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "Missing metadata")
	}

	authHeader, ok := md["authorization"]
	if !ok || len(authHeader) == 0 {
		return nil, status.Error(codes.Unauthenticated, "Missing authorization token")
	}

	// Fail closed: never compare against a built-in default token.
	expectedToken := internal.GetString("GRPCSERVER_TOKEN", "")
	if expectedToken == "" {
		return nil, status.Error(codes.Unauthenticated, "Server authorization token is not configured")
	}

	token := strings.TrimPrefix(authHeader[0], "Bearer ")
	if token != expectedToken {
		return nil, status.Error(codes.Unauthenticated, "Invalid authorization token")
	}

	return handler(ctx, req)
}
