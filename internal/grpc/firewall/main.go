package firewall

import (
	"github.com/werbot/werbot/internal"
	firewallpb "github.com/werbot/werbot/internal/grpc/firewall/proto/firewall"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
)

var (
	log     logger.Logger
	envMode string
)

// Handler is ...
type Handler struct {
	firewallpb.UnimplementedFirewallHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New()
	envMode = internal.GetString("ENV_MODE", "prod")
}
