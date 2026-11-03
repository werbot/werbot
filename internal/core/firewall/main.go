package firewall

import (
	firewallrpc "github.com/werbot/werbot/internal/core/firewall/proto/rpc"
	"github.com/werbot/werbot/pkg/logger"
	"github.com/werbot/werbot/pkg/storage/postgres"
)

var log logger.Logger

// Handler is ...
type Handler struct {
	firewallrpc.UnimplementedFirewallHandlersServer
	DB *postgres.Connect
}

func init() {
	log = logger.New()
}
