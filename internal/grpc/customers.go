package grpc

import (
	"context"
	"database/sql"

	subscriptionpb "github.com/werbot/werbot/api/proto/subscription"
)

// ListCustomers is ...
func (s *subscription) ListCustomers(ctx context.Context, in *subscriptionpb.ListCustomers_Request) (*subscriptionpb.ListCustomers_Response, error) {
	response := new(subscriptionpb.ListCustomers_Response)

	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := service.db.Conn.Query(`SELECT "user_id", "stripe_id" FROM "subscription_customer"` + sqlFooter)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	for rows.Next() {
		customer := new(subscriptionpb.Customer_Response)
		if err := rows.Scan(&customer.UserId, &customer.StripeId); err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			service.log.FromGRPC(err).Send()
			return nil, errServerError
		}
		response.Customers = append(response.Customers, customer)
	}
	defer rows.Close()

	// Total count for pagination
	err = service.db.Conn.QueryRow(`SELECT COUNT (*) FROM "subscription_customer"`).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	return response, nil
}

// Customer is ...
func (s *subscription) Customer(ctx context.Context, in *subscriptionpb.Customer_Request) (*subscriptionpb.Customer_Response, error) {
	response := new(subscriptionpb.Customer_Response)
	response.UserId = in.GetUserId()

	err := service.db.Conn.QueryRow(`SELECT "stripe_id" FROM "subscription_customer" WHERE "user_id" = $1`,
		in.GetUserId(),
	).Scan(&response.StripeId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	return response, nil
}

// AddCustomer is ...
func (s *subscription) AddCustomer(ctx context.Context, in *subscriptionpb.AddCustomer_Request) (*subscriptionpb.AddCustomer_Response, error) {
	response := new(subscriptionpb.AddCustomer_Response)

	err := service.db.Conn.QueryRow(`INSERT	INTO "subscription_customer" ("user_id", "stripe_id")	VALUES ($1, $2)`,
		in.GetUserId(),
		in.GetStripeId(),
	).Scan(&response.CustomerId)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	return response, nil
}

// UpdateCustomer is ...
func (s *subscription) UpdateCustomer(ctx context.Context, in *subscriptionpb.UpdateCustomer_Request) (*subscriptionpb.UpdateCustomer_Response, error) {
	response := new(subscriptionpb.UpdateCustomer_Response)

	data, err := service.db.Conn.Exec(`UPDATE "subscription_customer" SET "stripe_id" = $1 WHERE "user_id" = $2`,
		in.GetStripeId(),
		in.GetUserId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// DeleteCustomer is ...
func (s *subscription) DeleteCustomer(ctx context.Context, in *subscriptionpb.DeleteCustomer_Request) (*subscriptionpb.DeleteCustomer_Response, error) {
	response := new(subscriptionpb.DeleteCustomer_Response)

	data, err := service.db.Conn.Exec(`DELETE FROM "subscription_customer" WHERE "user_id" = $1`,
		in.GetUserId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}
