package grpc

/*
import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/werbot/werbot/internal/config"
	"github.com/werbot/werbot/internal/database"

	pb "github.com/werbot/werbot/internal/grpc/proto/firewall"
)

func init() {
	config.Load("../../configs/.env")

	var err error
	pgDSN := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=require",
		config.GetString("POSTGRES_USER", "werbot"),
		config.GetString("POSTGRES_PASSWORD", "postgresPassword"),
		config.GetString("POSTGRES_HOST", "localhost:5432"),
		config.GetString("POSTGRES_DB", "werbot"),
	)
	db, err = database.ConnectDB(&database.PgSQLConfig{
		DSN:             pgDSN,
		MaxConn:         config.GetInt("PSQLSERVER_MAX_CONN", 50),
		MaxIdleConn:     config.GetInt("PSQLSERVER_MAX_IDLEC_CONN", 10),
		MaxLifetimeConn: config.GetInt("PSQLSERVER_MAX_LIFETIME_CONN", 300),
	})
	if err != nil {
		fmt.Printf("Failed connect to database: %s", err)
	}
}

func Test_firewall_CheckIPAccess(t *testing.T) {
	ctx := context.Background()

	type fields struct {
		UnimplementedFirewallHandlersServer pb.UnimplementedFirewallHandlersServer
	}
	type args struct {
		ctx context.Context
		in  *pb.CheckIPAccessRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.CheckIPAccessResponse
		wantErr bool
	}{
		// "Local host IP address"
		{
			name: "CheckIPAccess_01",
			args: args{
				ctx: ctx,
				in: &pb.CheckIPAccessRequest{
					ClientIp: "127.0.0.1",
				},
			},
			want: &pb.CheckIPAccessResponse{
				Access:  true,
				Country: "",
			},
		},

		// IP address on active blacklist
		{
			name: "CheckIPAccess_02",
			args: args{
				ctx: ctx,
				in: &pb.CheckIPAccessRequest{
					ClientIp: "178.239.2.11",
				},
			},
			want: &pb.CheckIPAccessResponse{
				Access:  false,
				Country: "",
			},
		},

		// IP address is not blacklisted
		{
			name: "CheckIPAccess_03",
			args: args{
				ctx: ctx,
				in: &pb.CheckIPAccessRequest{
					ClientIp: "178.239.2.1",
				},
			},
			want: &pb.CheckIPAccessResponse{
				Access:  true,
				Country: "PL",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &firewall{
				UnimplementedFirewallHandlersServer: tt.fields.UnimplementedFirewallHandlersServer,
			}
			got, err := s.CheckIPAccess(tt.args.ctx, tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("firewall.CheckIPAccess() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("firewall.CheckIPAccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_firewall_CheckServerAccess(t *testing.T) {
	type fields struct {
		UnimplementedFirewallHandlersServer pb.UnimplementedFirewallHandlersServer
	}
	type args struct {
		ctx context.Context
		in  *pb.CheckServerAccessRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *pb.CheckIPAccessResponse
		wantErr bool
	}{
		{
			name: "The user has access to the server",
			args: args{
				ctx: context.Background(),
				in: &pb.CheckServerAccessRequest{
					ClientIp:  "178.239.2.1",
					Country:   "PL",
					AccountId: 1,
					UserId:    1,
				},
			},
			want: &pb.CheckIPAccessResponse{
				Access: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &firewall{
				UnimplementedFirewallHandlersServer: tt.fields.UnimplementedFirewallHandlersServer,
			}
			got, err := s.CheckServerAccess(tt.args.ctx, tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("firewall.CheckServerAccess() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("firewall.CheckServerAccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_blackListIP(t *testing.T) {
	type args struct {
		ip string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "Local host IP address",
			args: args{
				ip: "127.0.0.1",
			},
			want: false,
		},
		{
			name: "IP address on active blacklist",
			args: args{
				ip: "178.239.2.11",
			},
			want: true,
		},
		{
			name: "IP address is not blacklisted",
			args: args{
				ip: "178.239.2.1",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := blackListIP(tt.args.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("blackListIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("blackListIP() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/
