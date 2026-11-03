package worker

import (
	"context"

	"google.golang.org/grpc"

	firewallmessage "github.com/werbot/werbot/internal/core/firewall/proto/message"
	firewallrpc "github.com/werbot/werbot/internal/core/firewall/proto/rpc"
	"github.com/werbot/werbot/pkg/worker"
)

func cronUpdateFirewallList(grpcClient *grpc.ClientConn) worker.CronHandler {
	return func(ctx context.Context) error {
		log.Info().Msg("Update firewall list")

		rClient := firewallrpc.NewFirewallHandlersClient(grpcClient)
		if _, err := rClient.UpdateFirewallListData(ctx, &firewallmessage.UpdateFirewallListData_Request{}); err != nil {
			log.Error(err).Send()
			return err
		}

		return nil
	}
}
