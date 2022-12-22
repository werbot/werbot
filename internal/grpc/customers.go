package grpc

import (
	"context"
	"database/sql"

	pb_subscription "github.com/werbot/werbot/api/proto/subscription"
)

// ListCustomers is ...
func (s *subscription) ListCustomers(ctx context.Context, in *pb_subscription.ListCustomers_Request) (*pb_subscription.ListCustomers_Response, error) {
	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := service.db.Conn.Query(`SELECT
			"user_id",
			"stripe_id"
		FROM
			"subscription_customer"` + sqlFooter)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToSelect
	}

	customers := []*pb_subscription.Customer_Response{}
	for rows.Next() {
		customer := new(pb_subscription.Customer_Response)
		err = rows.Scan(
			&customer.UserId,
			&customer.StripeId,
		)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToScan
		}
		customers = append(customers, customer)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	err = service.db.Conn.QueryRow(`SELECT
			COUNT (*)
		FROM
			"subscription_customer"`).
		Scan(&total)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToScan
	}

	return &pb_subscription.ListCustomers_Response{
		Total:     total,
		Customers: customers,
	}, nil
}

// Customer is ...
func (s *subscription) Customer(ctx context.Context, in *pb_subscription.Customer_Request) (*pb_subscription.Customer_Response, error) {
	customer := new(pb_subscription.Customer_Response)
	customer.UserId = in.GetUserId()
	err := service.db.Conn.QueryRow(`SELECT
			"stripe_id"
		FROM
			"subscription_customer"
		WHERE
			"user_id" = $1`,
		in.GetUserId(),
	).Scan(&customer.StripeId)
	if err != nil {
		service.log.ErrorGRPC(err)
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		return nil, errFailedToScan
	}

	return customer, nil
}

// AddCustomer is ...
func (s *subscription) AddCustomer(ctx context.Context, in *pb_subscription.AddCustomer_Request) (*pb_subscription.AddCustomer_Response, error) {
	var id string
	err := service.db.Conn.QueryRow(`INSERT
		INTO "subscription_customer" ("user_id", "stripe_id")
		VALUES ($1, $2)`,
		in.GetUserId(),
		in.GetStripeId(),
	).Scan(&id)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToAdd
	}

	return &pb_subscription.AddCustomer_Response{
		CustomerId: id,
	}, nil
}

// UpdateCustomer is ...
func (s *subscription) UpdateCustomer(ctx context.Context, in *pb_subscription.UpdateCustomer_Request) (*pb_subscription.UpdateCustomer_Response, error) {
	data, err := service.db.Conn.Exec(`UPDATE "subscription_customer"
		SET
			"stripe_id" = $1
		WHERE
			"user_id" = $2`,
		in.GetStripeId(),
		in.GetUserId(),
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_subscription.UpdateCustomer_Response{}, nil
}

// DeleteCustomer is ...
func (s *subscription) DeleteCustomer(ctx context.Context, in *pb_subscription.DeleteCustomer_Request) (*pb_subscription.DeleteCustomer_Response, error) {
	data, err := service.db.Conn.Exec(`DELETE
		FROM
			"subscription_customer"
		WHERE
			"user_id" = $1`,
		in.GetUserId(),
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_subscription.DeleteCustomer_Response{}, nil
}
