package grpc

import (
	"context"

	"github.com/jackc/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb_subscription "github.com/werbot/werbot/api/proto/subscription"
)

type subscription struct {
	pb_subscription.UnimplementedSubscriptionHandlersServer
}

// GetSubscriptions is ...
func (s *subscription) GetSubscriptions(ctx context.Context, in *pb_subscription.ListSubscriptions_Request) (*pb_subscription.ListSubscriptions_Response, error) {
	sqlSearch := service.db.SQLAddWhere(in.GetQuery())
	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := service.db.Conn.Query(`SELECT
      "subscription"."id",
      "subscription"."customer_id",
      "user"."name" AS "customer_name",
      "subscription"."plan_id",
      "subscription_plan"."title" AS "plane_name",
      "subscription"."start_date",
      "subscription"."end_date",
      "subscription"."state",
      "subscription"."stripe_id"
    FROM
      "subscription"
      INNER JOIN "subscription_customer" ON "subscription"."customer_id" = "subscription_customer"."user_id"
      INNER JOIN "subscription_plan" ON "subscription"."plan_id" = "subscription_plan"."id"
      INNER JOIN "user" ON "subscription_customer"."user_id" = "user"."id"` + sqlSearch + sqlFooter)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToSelect
	}

	subscriptions := []*pb_subscription.Subscription_Response{}
	for rows.Next() {
		var startDate, endDate pgtype.Timestamp
		subscription := new(pb_subscription.Subscription_Response)
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
			service.log.ErrorGRPC(err)
			return nil, errFailedToScan
		}
		subscription.StartDate = timestamppb.New(startDate.Time)
		subscription.EndDate = timestamppb.New(endDate.Time)
		subscriptions = append(subscriptions, subscription)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	err = service.db.Conn.QueryRow(`SELECT
      COUNT(*)
    FROM
      "subscription"
      INNER JOIN "user" ON "subscription"."customer_id" = "user"."id"` + sqlSearch).
		Scan(&total)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToScan
	}

	return &pb_subscription.ListSubscriptions_Response{
		Total:         total,
		Subscriptions: subscriptions,
	}, nil
}

// TODO Subscription is ...
func (s *subscription) GetSubscription(ctx context.Context, in *pb_subscription.Subscription_Request) (*pb_subscription.Subscription_Response, error) {
	return &pb_subscription.Subscription_Response{}, nil
}

// TODO AddSubscription is ...
func (s *subscription) AddSubscription(ctx context.Context, in *pb_subscription.AddSubscription_Request) (*pb_subscription.AddSubscription_Response, error) {
	return &pb_subscription.AddSubscription_Response{}, nil
}

// TODO UpdateSubscription is ...
func (s *subscription) UpdateSubscription(ctx context.Context, in *pb_subscription.UpdateSubscription_Request) (*pb_subscription.UpdateSubscription_Response, error) {
	return &pb_subscription.UpdateSubscription_Response{}, nil
}

// DeleteSubscription is ...
func (s *subscription) DeleteSubscription(ctx context.Context, in *pb_subscription.DeleteSubscription_Request) (*pb_subscription.DeleteSubscription_Response, error) {
	data, err := service.db.Conn.Exec(`DELETE
		FROM
			"subscription"
		WHERE
			"id" = $1`,
		in.SubscriptionId,
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_subscription.DeleteSubscription_Response{}, nil
}

// TODO Changes is ...
func (s *subscription) GetChanges(ctx context.Context, in *pb_subscription.Changes_Request) (*pb_subscription.Changes_Response, error) {
	return &pb_subscription.Changes_Response{}, nil
}

// TODO Invoices is ...
func (s *subscription) GetInvoices(ctx context.Context, in *pb_subscription.Invoices_Request) (*pb_subscription.Invoices_Response, error) {
	return &pb_subscription.Invoices_Response{}, nil
}
