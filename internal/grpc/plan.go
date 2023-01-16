package grpc

import (
	"context"
	"database/sql"

	"github.com/jackc/pgtype"

	subscriptionpb "github.com/werbot/werbot/api/proto/subscription"
)

// ListPlans is ...
func (p *subscription) ListPlans(ctx context.Context, in *subscriptionpb.ListPlans_Request) (*subscriptionpb.ListPlans_Response, error) {
	response := new(subscriptionpb.ListPlans_Response)

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
			(SELECT COUNT(*) FROM "subscription" WHERE plan_id = "subscription_plan"."id" ) AS count_subscriptions
		FROM "subscription_plan"` + sqlSearch + sqlFooter)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	for rows.Next() {
		var countSubscription int32
		plan := new(subscriptionpb.ListPlans_PlanInfo)
		err = rows.Scan(&plan.Plan.PlanId,
			&plan.Plan.Cost,
			&plan.Plan.Period,
			&plan.Plan.Title,
			&plan.Plan.Active,
			&plan.Plan.Trial,
			&plan.Plan.TrialPeriod,
			&plan.Plan.LimitsServers,
			&plan.Plan.LimitsUsers,
			&plan.Plan.LimitsCompanies,
			&plan.Plan.LimitsConnections,
			&plan.Plan.Default,
			&countSubscription,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			service.log.FromGRPC(err).Send()
			return nil, errServerError
		}
		plan.SubscriptionCount = countSubscription

		response.Plans = append(response.Plans, plan)
	}
	defer rows.Close()

	// Total records for pagination
	err = service.db.Conn.QueryRow(`SELECT COUNT (*) FROM "subscription_plan"` + sqlSearch).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	return response, nil
}

// Plan is ...
func (p *subscription) Plan(ctx context.Context, in *subscriptionpb.Plan_Request) (*subscriptionpb.Plan_Response, error) {
	var allowedSections, benefits pgtype.JSON
	response := new(subscriptionpb.Plan_Response)
	response.PlanId = in.GetPlanId()

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
		FROM "subscription_plan"
		WHERE "id" = $1`,
		in.GetPlanId(),
	).Scan(&response.Cost,
		&response.Period,
		&response.Title,
		&response.StripeId,
		&allowedSections,
		&benefits,
		&response.Image,
		&response.Active,
		&response.Trial,
		&response.TrialPeriod,
		&response.LimitsServers,
		&response.LimitsUsers,
		&response.LimitsCompanies,
		&response.LimitsConnections,
		&response.Default,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	var jSections []string
	allowedSections.AssignTo(&jSections)
	response.AllowedSections = jSections

	var jBenefits map[int32]string
	benefits.AssignTo(&jBenefits)
	response.Benefits = jBenefits

	return response, nil
}

// UpdatePlan is ...
func (p *subscription) UpdatePlan(ctx context.Context, in *subscriptionpb.UpdatePlan_Request) (*subscriptionpb.UpdatePlan_Response, error) {
	response := new(subscriptionpb.UpdatePlan_Response)

	data, err := service.db.Conn.Exec(`UPDATE "subscription_plan"
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
		WHERE "id" = $16`,
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
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	// update Starter plan
	if in.Default {
		data, err := service.db.Conn.Exec(`UPDATE "subscription_plan" SET "default" = false WHERE "id" != $1`,
			in.GetPlanId(),
		)
		if err != nil {
			service.log.FromGRPC(err).Send()
			return nil, errFailedToUpdate
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}
	}

	return response, nil
}
