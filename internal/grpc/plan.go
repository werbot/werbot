package grpc

import (
	"context"
	"errors"

	"github.com/jackc/pgtype"

	pb_subscription "github.com/werbot/werbot/internal/grpc/proto/subscription"
)

// GetSubscriptionPlans is ...
func (p *subscription) GetSubscriptionPlans(ctx context.Context, in *pb_subscription.ListPlans_Request) (*pb_subscription.ListPlans_Response, error) {
	sqlSearch := db.SQLAddWhere(in.GetQuery())
	sqlFooter := db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())

	rows, err := db.Conn.Query(`SELECT
			"id",
			"cost",
			"period",
			"title",
			"active",
			"trial",
			"trial_period",
			"limits_servers",
			"limits_users",
			"limits_companies",
			"limits_connections",
			"default",
			(SELECT COUNT(*) FROM subscription WHERE plan_id = "subscription_plan"."id" ) AS count_subscriptions
		FROM
			"subscription_plan"` + sqlSearch + sqlFooter)
	if err != nil {
		//return nil, errors.New("ListSubscriptionPlans failed")
		return nil, err
	}

	plans := []*pb_subscription.ListPlans_Response_PlanInfo{}
	for rows.Next() {
		plan := pb_subscription.GetPlan_Response{}
		var countSubscription int32

		err = rows.Scan(&plan.PlanId,
			&plan.Cost,
			&plan.Period,
			&plan.Title,
			&plan.Active,
			&plan.Trial,
			&plan.TrialPeriod,
			&plan.LimitsServers,
			&plan.LimitsUsers,
			&plan.LimitsCompanies,
			&plan.LimitsConnections,
			&plan.Default,
			&countSubscription,
		)
		if err != nil {
			//return nil, errors.New("ListSubscriptionPlans Scan failed")
			return nil, err
		}

		plans = append(plans, &pb_subscription.ListPlans_Response_PlanInfo{
			SubscriptionCount: countSubscription,
			Plan:              &plan,
		})
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	db.Conn.QueryRow(`SELECT COUNT (*) FROM "subscription_plan"` + sqlSearch).Scan(&total)

	return &pb_subscription.ListPlans_Response{
		Total: total,
		Plans: plans,
	}, nil
}

// GetSubscriptionPlan is ...
func (p *subscription) GetSubscriptionPlan(ctx context.Context, in *pb_subscription.GetPlan_Request) (*pb_subscription.GetPlan_Response, error) {
	var allowedSections, benefits pgtype.JSON
	plan := pb_subscription.GetPlan_Response{}
	plan.PlanId = in.GetPlanId()

	err := db.Conn.QueryRow(`SELECT
			"cost",
			"period",
			"title",
			"stripe_id",
			"allowed_sections",
			"benefits",
			"image",
			"active",
			"trial",
			"trial_period",
			"limits_servers",
			"limits_users",
			"limits_companies",
			"limits_connections",
			"default" 
		FROM
			"subscription_plan" 
		WHERE
			"id" = $1`, in.GetPlanId()).
		Scan(
			&plan.Cost,
			&plan.Period,
			&plan.Title,
			&plan.StripeId,
			&allowedSections,
			&benefits,
			&plan.Image,
			&plan.Active,
			&plan.Trial,
			&plan.TrialPeriod,
			&plan.LimitsServers,
			&plan.LimitsUsers,
			&plan.LimitsCompanies,
			&plan.LimitsConnections,
			&plan.Default,
		)
	if err != nil {
		//return nil, errors.New("GetSubscriptionPlan failed")
		return nil, err
	}

	var a []string
	allowedSections.AssignTo(&a)
	plan.AllowedSections = a

	var b map[int32]string
	benefits.AssignTo(&b)
	plan.Benefits = b

	return &plan, nil
}

// UpdateSubscriptionPlan is ...
func (p *subscription) UpdateSubscriptionPlan(ctx context.Context, in *pb_subscription.UpdatePlan_Request) (*pb_subscription.UpdatePlan_Response, error) {
	_, err := db.Conn.Exec(`UPDATE "subscription_plan" 
		SET "cost" = $1,
			"period" = $2,
			"title" = $3,
			"stripe_id" = $4,
			"allowed_sections" = $5,
			"benefits" = $6,
			"image" = $7,
			"active" = $8,
			"trial" = $9,
			"trial_period" = $10,
			"limits_servers" = $11,
			"limits_users" = $12,
			"limits_companies" = $13,
			"limits_connections" = $14,
			"default" = $15 
		WHERE 
			"id" = $16`,
		in.GetCost(),
		in.GetPeriod(),
		in.GetTitle(),
		in.GetStripeId(),
		in.GetAllowedSections(),
		in.GetBenefits(),
		in.GetImage(),
		in.GetActive(),
		in.GetTrial(),
		in.GetTrialPeriod(),
		in.GetLimitsServers(),
		in.GetLimitsUsers(),
		in.GetLimitsCompanies(),
		in.GetLimitsConnections(),
		in.GetDefault(),
		in.GetPlanId(),
	)
	if err != nil {
		//return nil, errors.New("UpdateSubscriptionPlan failed")
		return nil, err
	}

	// update Starter plan
	if in.Default {
		if _, err := db.Conn.Query(`UPDATE "subscription_plan" SET "default" = false WHERE "id" != $1`, in.GetPlanId()); err != nil {
			return nil, errors.New("Update default plan failed")
		}
	}

	return &pb_subscription.UpdatePlan_Response{}, nil
}
