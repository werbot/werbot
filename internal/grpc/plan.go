package grpc

import (
	"context"

	"github.com/jackc/pgtype"

	pb_subscription "github.com/werbot/werbot/api/proto/subscription"
)

// ListPlans is ...
func (p *subscription) ListPlans(ctx context.Context, in *pb_subscription.ListPlans_Request) (*pb_subscription.ListPlans_Response, error) {
	sqlSearch := service.db.SQLAddWhere(in.GetQuery())
	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := service.db.Conn.Query(`SELECT
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
		return nil, errFailedToSelect
	}

	plans := []*pb_subscription.ListPlans_Response_PlanInfo{}
	for rows.Next() {
		var countSubscription int32
		plan := new(pb_subscription.Plan_Response)
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
			return nil, errFailedToScan
		}

		plans = append(plans, &pb_subscription.ListPlans_Response_PlanInfo{
			SubscriptionCount: countSubscription,
			Plan:              plan,
		})
	}
	defer rows.Close()

	var total int32
	err = service.db.Conn.QueryRow(`SELECT COUNT (*) FROM "subscription_plan"` + sqlSearch).Scan(&total)
	if err != nil {
		return nil, errFailedToScan
	}

	return &pb_subscription.ListPlans_Response{
		Total: total,
		Plans: plans,
	}, nil
}

// Plan is ...
func (p *subscription) Plan(ctx context.Context, in *pb_subscription.Plan_Request) (*pb_subscription.Plan_Response, error) {
	var allowedSections, benefits pgtype.JSON
	plan := new(pb_subscription.Plan_Response)
	plan.PlanId = in.GetPlanId()

	err := service.db.Conn.QueryRow(`SELECT
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
		return nil, errFailedToScan
	}

	var jSections []string
	allowedSections.AssignTo(&jSections)
	plan.AllowedSections = jSections

	var jBenefits map[int32]string
	benefits.AssignTo(&jBenefits)
	plan.Benefits = jBenefits

	return plan, nil
}

// UpdatePlan is ...
func (p *subscription) UpdatePlan(ctx context.Context, in *pb_subscription.UpdatePlan_Request) (*pb_subscription.UpdatePlan_Response, error) {
	data, err := service.db.Conn.Exec(`UPDATE
      "subscription_plan"
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
		return nil, errFailedToScan
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	// update Starter plan
	if in.Default {
		data, err := service.db.Conn.Exec(`UPDATE
      "subscription_plan"
		SET
			"default" = false
		WHERE
			"id" != $1`,
			in.GetPlanId(),
		)
		if err != nil {
			return nil, errFailedToUpdate
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}
	}

	return &pb_subscription.UpdatePlan_Response{}, nil
}
