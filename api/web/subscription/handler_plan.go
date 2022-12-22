package subscription

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/utils/validate"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/api/proto/subscription"
)

// @Summary      List of all tariff plans
// @Tags         plans
// @Accept       json
// @Produce      json
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/subscriptions/plans [get]
func (h *handler) getSubscriptionPlans(c *fiber.Ctx) error {
	userParameter := middleware.AuthUser(c)
	pagination := httputil.GetPaginationFromCtx(c)
	sqlQuery := ""

	if !userParameter.IsUserAdmin() {
		sqlQuery = `"active"=true`
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewSubscriptionHandlersClient(h.Grpc.Client)

	plans, err := rClient.ListPlans(ctx, &pb.ListPlans_Request{
		Query:  sqlQuery,
		Limit:  pagination.GetLimit(),
		Offset: pagination.GetOffset(),
		SortBy: "id:ASC",
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}
	if plans.GetTotal() == 0 {
		return httputil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	if userParameter.IsUserAdmin() {
		return httputil.StatusOK(c, "Tariff plans", plans)
	}

	// response info for ROLE_USER
	planLite := []*pb.PlansLite_PlanLite{}
	for _, s := range plans.Plans {
		plan := pb.PlansLite_PlanLite{
			PlanId:            s.Plan.GetPlanId(),
			Cost:              s.Plan.GetCost(),
			Period:            s.Plan.GetPeriod(),
			Title:             s.Plan.GetTitle(),
			Image:             s.Plan.GetImage(),
			TrialPeriod:       s.Plan.GetTrialPeriod(),
			LimitsServers:     s.Plan.GetLimitsServers(),
			LimitsUsers:       s.Plan.GetLimitsUsers(),
			LimitsCompanies:   s.Plan.GetLimitsCompanies(),
			LimitsConnections: s.Plan.GetLimitsConnections(),
		}

		planLite = append(planLite, &plan)
	}

	return httputil.StatusOK(c, "Tariff plans", pb.PlansLite{
		Total: plans.GetTotal(),
		Plans: planLite,
	})
}

// @Summary      Information about the tariff plan
// @Tags         plans
// @Accept       json
// @Produce      json
// @Param        plan_id     path     int true "plan_id"
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/subscriptions/plans/:plan_id [get]
func (h *handler) getSubscriptionPlan(c *fiber.Ctx) error {
	planID := c.Params("plan_id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewSubscriptionHandlersClient(h.Grpc.Client)

	plan, err := rClient.Plan(ctx, &pb.Plan_Request{
		PlanId: planID,
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	userParameter := middleware.AuthUser(c)
	if userParameter.IsUserAdmin() {
		// response info for ROLE_ADMIN
		return httputil.StatusOK(c, "Information about the tariff plan", plan)
	}

	// response info for ROLE_USER
	return httputil.StatusOK(c, "Information about the tariff plan", pb.PlansLite_PlanLite{
		PlanId:            plan.GetPlanId(),
		Cost:              plan.GetCost(),
		Period:            plan.GetPeriod(),
		Title:             plan.GetTitle(),
		Image:             plan.GetImage(),
		TrialPeriod:       plan.GetTrialPeriod(),
		LimitsServers:     plan.GetLimitsServers(),
		LimitsUsers:       plan.GetLimitsUsers(),
		LimitsCompanies:   plan.GetLimitsCompanies(),
		LimitsConnections: plan.GetLimitsConnections(),
	})
}

// @Summary      Tariff plan update by its ID
// @Tags         plans
// @Accept       json
// @Produce      json
// @Param        req         body     pb.Plan
// @Param        key_id      path     int true "key_id"
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/subscriptions/plans/:plan_id [patch]
func (h *handler) patchSubscriptionPlan(c *fiber.Ctx) error {
	planID := c.Params("plan_id")
	input := new(pb.UpdatePlan_Request)

	if err := c.BodyParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	userParameter := middleware.AuthUser(c)
	if !userParameter.IsUserAdmin() {
		return httputil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewSubscriptionHandlersClient(h.Grpc.Client)

	_, err := rClient.UpdatePlan(ctx, &pb.UpdatePlan_Request{
		PlanId:            planID,
		Cost:              input.GetCost(),
		Period:            input.GetPeriod(),
		Title:             input.GetTitle(),
		StripeId:          input.GetStripeId(),
		AllowedSections:   input.GetAllowedSections(),
		Benefits:          input.GetBenefits(),
		Image:             input.GetImage(),
		Active:            input.GetActive(),
		Trial:             input.GetTrial(),
		TrialPeriod:       input.GetTrialPeriod(),
		LimitsServers:     input.GetLimitsServers(),
		LimitsUsers:       input.GetLimitsServers(),
		LimitsCompanies:   input.GetLimitsCompanies(),
		LimitsConnections: input.GetLimitsConnections(),
		Default:           input.GetDefault(),
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, "Tariff plan updated successfully", nil)
}
