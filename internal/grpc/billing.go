package grpc

import (
	"context"

	pb_billing "github.com/werbot/werbot/api/proto/billing"
)

type billing struct {
	pb_billing.UnimplementedBillingHandlersServer
}

// TODO UpdateProduct is ...
func (b *billing) UpdateProduct(ctx context.Context, in *pb_billing.UpdateProduct_Request) (*pb_billing.UpdateProduct_Response, error) {
	return &pb_billing.UpdateProduct_Response{}, nil
}
