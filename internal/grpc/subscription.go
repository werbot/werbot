package grpc

import (
	"context"
	"errors"

	"github.com/jackc/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb_subscription "github.com/werbot/werbot/api/proto/subscription"
)

type subscription struct {
	pb_subscription.UnimplementedSubscriptionHandlersServer
}

// GetSubscriptions is ...
func (s *subscription) GetSubscriptions(ctx context.Context, in *pb_subscription.ListSubscriptions_Request) (*pb_subscription.ListSubscriptions_Response, error) {
	sqlSearch := db.SQLAddWhere(in.GetQuery())
	sqlFooter := db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())

	rows, err := db.Conn.Query(`SELECT
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
		return nil, errors.New("ListSubscriptionPlans failed")
	}

	subscriptions := []*pb_subscription.GetSubscription_Response{}
	//var count int32
	for rows.Next() {
		subscription := pb_subscription.GetSubscription_Response{}
		var startDate, endDate pgtype.Timestamp

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
			return nil, errors.New("GetSubscriptions Scan")
		}

		subscription.StartDate = timestamppb.New(startDate.Time)
		subscription.EndDate = timestamppb.New(endDate.Time)

		subscriptions = append(subscriptions, &subscription)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	db.Conn.QueryRow(`SELECT COUNT(*)
    FROM
      "subscription"
      INNER JOIN "user" ON "subscription"."customer_id" = "user"."id"` + sqlSearch).Scan(&total)

	return &pb_subscription.ListSubscriptions_Response{
		Total:         total,
		Subscriptions: subscriptions,
	}, nil
}

// TODO GetSubscription is ...
func (s *subscription) GetSubscription(ctx context.Context, in *pb_subscription.GetSubscription_Request) (*pb_subscription.GetSubscription_Response, error) {
	return &pb_subscription.GetSubscription_Response{}, nil
}

// TODO CreateSubscription is ...
func (s *subscription) CreateSubscription(ctx context.Context, in *pb_subscription.CreateSubscription_Request) (*pb_subscription.CreateSubscription_Response, error) {
	return &pb_subscription.CreateSubscription_Response{}, nil
}

// TODO UpdateSubscription is ...
func (s *subscription) UpdateSubscription(ctx context.Context, in *pb_subscription.UpdateSubscription_Request) (*pb_subscription.UpdateSubscription_Response, error) {
	return &pb_subscription.UpdateSubscription_Response{}, nil
}

// DeleteSubscription is ...
func (s *subscription) DeleteSubscription(ctx context.Context, in *pb_subscription.DeleteSubscription_Request) (*pb_subscription.DeleteSubscription_Response, error) {
	_, err := db.Conn.Exec(`DELETE 
		FROM 
			"subscription" 
		WHERE 
			"id" = $1`,
		in.SubscriptionId,
	)
	if err != nil {
		return &pb_subscription.DeleteSubscription_Response{}, errors.New("DeleteSubscription failed")
	}

	return &pb_subscription.DeleteSubscription_Response{}, nil
}

// TODO GetChanges is ...
func (s *subscription) GetChanges(ctx context.Context, in *pb_subscription.GetChanges_Request) (*pb_subscription.GetChanges_Response, error) {
	return &pb_subscription.GetChanges_Response{}, nil
}

// TODO GetInvoices is ...
func (s *subscription) GetInvoices(ctx context.Context, in *pb_subscription.GetInvoices_Request) (*pb_subscription.GetInvoices_Response, error) {
	return &pb_subscription.GetInvoices_Response{}, nil
}
