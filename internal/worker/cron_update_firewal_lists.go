package worker

import (
	"context"

	"google.golang.org/grpc"

	firewallpb "github.com/werbot/werbot/internal/core/firewall/proto/firewall"
	"github.com/werbot/werbot/pkg/worker"
)

func cronUpdateFirewallList(grpcClient *grpc.ClientConn) worker.CronHandler {
	return func(_ context.Context) error {
		log.Info().Msg("Update firewall list")

		rClient := firewallpb.NewFirewallHandlersClient(grpcClient)
		if _, err := rClient.UpdateFirewallListData(context.Background(), nil); err != nil {
			log.Fatal(err).Send()
		}

		return nil
	}
}
