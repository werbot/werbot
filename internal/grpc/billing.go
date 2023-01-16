package grpc

import (
	"context"

	billingpb "github.com/werbot/werbot/api/proto/billing"
)

type billing struct {
	billingpb.UnimplementedBillingHandlersServer
}

// TODO Product is ...
func (b *billing) Product(ctx context.Context, in *billingpb.Product_Request) (*billingpb.Product_Response, error) {
	response := new(billingpb.Product_Response)
	return response, nil
}

// TODO UpdateProduct is ...
func (b *billing) UpdateProduct(ctx context.Context, in *billingpb.UpdateProduct_Request) (*billingpb.UpdateProduct_Response, error) {
	response := new(billingpb.UpdateProduct_Response)
	return response, nil
}
