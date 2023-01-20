package grpc

import (
	"context"
	"database/sql"

	"github.com/jackc/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	subscriptionpb "github.com/werbot/werbot/api/proto/subscription"
)

type subscription struct {
	subscriptionpb.UnimplementedSubscriptionHandlersServer
}

// ListSubscriptions is ...
func (s *subscription) ListSubscriptions(ctx context.Context, in *subscriptionpb.ListSubscriptions_Request) (*subscriptionpb.ListSubscriptions_Response, error) {
	response := new(subscriptionpb.ListSubscriptions_Response)

	sqlSearch := service.db.SQLAddWhere(in.GetQuery())
	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := service.db.Conn.Query(`SELECT
      "subscription"."id",
      "subscription"."customer_id",
      "user"."login" AS "customer_name",
      "subscription"."plan_id",
      "subscription_plan"."title" AS "plane_name",
      "subscription"."start_date",
      "subscription"."end_date",
      "subscription"."state",
      "subscription"."stripe_id"
    FROM "subscription"
      INNER JOIN "subscription_customer" ON "subscription"."customer_id" = "subscription_customer"."user_id"
      INNER JOIN "subscription_plan" ON "subscription"."plan_id" = "subscription_plan"."id"
      INNER JOIN "user" ON "subscription_customer"."user_id" = "user"."id"` + sqlSearch + sqlFooter)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	for rows.Next() {
		var startDate, endDate pgtype.Timestamp
		subscription := new(subscriptionpb.Subscription_Response)
		err = rows.Scan(&subscription.SubscriptionId,
			&subscription.CustomerId,
			&subscription.CustomerName,
			&subscription.PlanId,
			&subscription.PlanName,
			&startDate,
			&endDate,
			&subscription.State,
			&subscription.StripeId,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			service.log.FromGRPC(err).Send()
			return nil, errServerError
		}
		subscription.StartDate = timestamppb.New(startDate.Time)
		subscription.EndDate = timestamppb.New(endDate.Time)

		response.Subscriptions = append(response.Subscriptions, subscription)
	}
	defer rows.Close()

	// Total count for pagination
	err = service.db.Conn.QueryRow(`SELECT COUNT(*)
    FROM "subscription"
      INNER JOIN "user" ON "subscription"."customer_id" = "user"."id"` + sqlSearch,
	).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	return response, nil
}

// TODO Subscription is ...
func (s *subscription) Subscription(ctx context.Context, in *subscriptionpb.Subscription_Request) (*subscriptionpb.Subscription_Response, error) {
	response := new(subscriptionpb.Subscription_Response)
	return response, nil
}

// TODO AddSubscription is ...
func (s *subscription) AddSubscription(ctx context.Context, in *subscriptionpb.AddSubscription_Request) (*subscriptionpb.AddSubscription_Response, error) {
	response := new(subscriptionpb.AddSubscription_Response)
	return response, nil
}

// TODO UpdateSubscription is ...
func (s *subscription) UpdateSubscription(ctx context.Context, in *subscriptionpb.UpdateSubscription_Request) (*subscriptionpb.UpdateSubscription_Response, error) {
	response := new(subscriptionpb.UpdateSubscription_Response)
	return response, nil
}

// DeleteSubscription is ...
func (s *subscription) DeleteSubscription(ctx context.Context, in *subscriptionpb.DeleteSubscription_Request) (*subscriptionpb.DeleteSubscription_Response, error) {
	response := new(subscriptionpb.DeleteSubscription_Response)

	data, err := service.db.Conn.Exec(`DELETE FROM "subscription" WHERE	"id" = $1`,
		in.GetSubscriptionId(),
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

// TODO ListChanges is ...
func (s *subscription) ListChanges(ctx context.Context, in *subscriptionpb.ListChanges_Request) (*subscriptionpb.ListChanges_Response, error) {
	response := new(subscriptionpb.ListChanges_Response)
	return response, nil
}

// TODO Change is ...
func (s *subscription) Change(ctx context.Context, in *subscriptionpb.Change_Request) (*subscriptionpb.Change_Response, error) {
	response := new(subscriptionpb.Change_Response)
	return response, nil
}

// TODO ListInvoices is ...
func (s *subscription) ListInvoices(ctx context.Context, in *subscriptionpb.ListInvoices_Request) (*subscriptionpb.ListInvoices_Response, error) {
	response := new(subscriptionpb.ListInvoices_Response)
	return response, nil
}

// TODO Invoice is ...
func (s *subscription) Invoice(ctx context.Context, in *subscriptionpb.Invoice_Request) (*subscriptionpb.Invoice_Response, error) {
	response := new(subscriptionpb.Invoice_Response)
	return response, nil
}
