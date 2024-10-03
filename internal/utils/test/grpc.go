package test

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
)

// GRPCTable is ...
type GRPCTable struct {
	Debug       bool
	Name        string       // The name of the test
	PreWorkHook func()       // A hook function to be executed before the test
	Request     ProtoMessage // The request payload for the test case
	Response    BodyTable    // The expected response payload for the test case
	Error       ErrGRPC      // The expected error message, if any, for the test case
}

// ErrGRPC  is ...
type ErrGRPC struct {
	Code    codes.Code
	Message any
}

// GRPCHandler is ...
type GRPCHandler struct {
	*grpc.ClientConn
}

// GRPC is ...
func GRPC(t *testing.T, addonDirs ...string) (*GRPCHandler, func(t *testing.T)) {
	t.Setenv("ENV_MODE", "test")

	migrationsDirs := []string{"../../../migration"}
	fixturesDirs := []string{"../../../fixtures/migration"}
	if len(addonDirs) > 0 {
		for _, dir := range addonDirs {
			migrationsDirs = append(migrationsDirs, dir+"migration")
			fixturesDirs = append(migrationsDirs, dir+"fixtures/migration")
		}
	}

	migrations := append(migrationsDirs, fixturesDirs...)
	pgTest := ServerPostgres(t, migrations...)

	redisTest := ServerRedis(context.Background(), t)

	grpcTest, err := ServerGRPC(context.Background(), t, pgTest, redisTest)
	if err != nil {
		t.Error(err)
	}

	return &GRPCHandler{
			grpcTest.ClientConn,
		}, func(_ *testing.T) {
			grpcTest.Close()
			redisTest.Close()
		}
}

func convertProtoToBodyTable(proto ProtoMessage) (BodyTable, error) {
	testResponse, err := protojson.MarshalOptions{
		UseEnumNumbers: true,
		UseProtoNames:  true,
		Multiline:      true,
	}.Marshal(proto)
	if err != nil {
		return BodyTable{}, err
	}
	var resp BodyTable
	if err := json.Unmarshal(testResponse, &resp); err != nil {
		return BodyTable{}, err
	}
	return resp, nil
}
