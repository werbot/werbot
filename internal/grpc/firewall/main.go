package firewall

import (
	"github.com/werbot/werbot/internal"
	firewallpb "github.com/werbot/werbot/internal/grpc/firewall/proto"
	"github.com/werbot/werbot/internal/storage/postgres"
	"github.com/werbot/werbot/pkg/logger"
)

var (
	devMode bool
)

// Handler is ...
type Handler struct {
	firewallpb.UnimplementedFirewallHandlersServer
	DB  *postgres.Connect
	Log logger.Logger
}

func init() {
	devMode = internal.GetBool("DEV_MODE", false)
}
