package grpc

import (
	"context"
	"errors"

	pb_subscription "github.com/werbot/werbot/api/proto/subscription"
)

// GetCustomers is ...
func (s *subscription) GetCustomers(ctx context.Context, in *pb_subscription.ListCustomers_Request) (*pb_subscription.ListCustomers_Response, error) {
	sqlFooter := db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())

	rows, err := db.Conn.Query(`SELECT 
			"user_id", 
			"stripe_id" 
		FROM 
			"subscription_customer"` + sqlFooter)
	if err != nil {
		return nil, errors.New("ListCustomers failed")
	}

	customers := []*pb_subscription.GetCustomer_Response{}
	//var count int32

	for rows.Next() {
		customer := pb_subscription.GetCustomer_Response{}

		err = rows.Scan(
			&customer.UserId,
			&customer.StripeId,
		)
		if err != nil {
			return nil, errors.New("ListCustomers Scan")
		}

		customers = append(customers, &customer)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	db.Conn.QueryRow(`SELECT COUNT (*) 
		FROM 
			"subscription_customer"`).Scan(&total)

	return &pb_subscription.ListCustomers_Response{
		Total:     total,
		Customers: customers,
	}, nil
}

// GetCustomer is ...
func (s *subscription) GetCustomer(ctx context.Context, in *pb_subscription.GetCustomer_Request) (*pb_subscription.GetCustomer_Response, error) {
	customer := pb_subscription.GetCustomer_Response{}
	customer.UserId = in.GetUserId()

	if err := db.Conn.QueryRow(`SELECT 
			"stripe_id" 
		FROM 
			"subscription_customer" 
		WHERE 
			"user_id" = $1`,
		in.GetUserId(),
	).Scan(&customer.StripeId); err != nil {
		return nil, errors.New("GetCustomer failed")
	}

	return &customer, nil
}

// CreateCustomer is ...
func (s *subscription) CreateCustomer(ctx context.Context, in *pb_subscription.CreateCustomer_Request) (*pb_subscription.CreateCustomer_Response, error) {
	var id string
	err := db.Conn.QueryRow(`INSERT 
		INTO "subscription_customer" ("user_id", "stripe_id") 
		VALUES ($1, $2)`,
		in.GetUserId(),
		in.GetStripeId(),
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	return &pb_subscription.CreateCustomer_Response{
		CustomerId: id,
	}, nil
}

// UpdateCustomer is ...
func (s *subscription) UpdateCustomer(ctx context.Context, in *pb_subscription.UpdateCustomer_Request) (*pb_subscription.UpdateCustomer_Response, error) {
	_, err := db.Conn.Exec(`UPDATE "subscription_customer" 
		SET 
			"stripe_id" = $1 
		WHERE 
			"user_id" = $2`,
		in.GetStripeId(),
		in.GetUserId(),
	)
	if err != nil {
		return nil, errors.New("UpdateCustomer failed")
	}

	return &pb_subscription.UpdateCustomer_Response{}, nil
}

// DeleteCustomer is ...
func (s *subscription) DeleteCustomer(ctx context.Context, in *pb_subscription.DeleteCustomer_Request) (*pb_subscription.DeleteCustomer_Response, error) {
	_, err := db.Conn.Exec(`DELETE 
		FROM 
			"subscription_customer"
		WHERE 
			"user_id" = $1`,
		in.GetUserId(),
	)
	if err != nil {
		return nil, errors.New("DeleteCustomer failed")
	}

	return &pb_subscription.DeleteCustomer_Response{}, nil
}
