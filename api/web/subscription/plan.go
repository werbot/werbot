package subscription

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	subscriptionpb "github.com/werbot/werbot/api/proto/subscription"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary      List of all tariff plans
// @Tags         plans
// @Accept       json
// @Produce      json
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/subscriptions/plans [get]
func (h *Handler) getSubscriptionPlans(c *fiber.Ctx) error {
	userParameter := middleware.AuthUser(c)
	sqlQuery := ""

	if !userParameter.IsUserAdmin() {
		sqlQuery = `"active"=true`
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pagination := webutil.GetPaginationFromCtx(c)

	rClient := subscriptionpb.NewSubscriptionHandlersClient(h.Grpc.Client)
	plans, err := rClient.ListPlans(ctx, &subscriptionpb.ListPlans_Request{
		Query:  sqlQuery,
		Limit:  pagination.GetLimit(),
		Offset: pagination.GetOffset(),
		SortBy: "id:ASC",
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}
	if plans.GetTotal() == 0 {
		return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	if userParameter.IsUserAdmin() {
		return webutil.StatusOK(c, msgPlans, plans)
	}

	// response info for ROLE_USER
	planLite := []*subscriptionpb.PlansLite_PlanLite{}
	for _, s := range plans.Plans {
		plan := subscriptionpb.PlansLite_PlanLite{
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

	return webutil.StatusOK(c, msgPlans, subscriptionpb.PlansLite{
		Total: plans.GetTotal(),
		Plans: planLite,
	})
}

// @Summary      Information about the tariff plan
// @Tags         plans
// @Accept       json
// @Produce      json
// @Param        plan_id     path     int true "plan_id"
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/subscriptions/plans/:plan_id [get]
func (h *Handler) getSubscriptionPlan(c *fiber.Ctx) error {
	planID := c.Params("plan_id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := subscriptionpb.NewSubscriptionHandlersClient(h.Grpc.Client)

	plan, err := rClient.Plan(ctx, &subscriptionpb.Plan_Request{
		PlanId: planID,
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	userParameter := middleware.AuthUser(c)
	if userParameter.IsUserAdmin() {
		// response info for ROLE_ADMIN
		return webutil.StatusOK(c, msgPlanInfo, plan)
	}

	// response info for ROLE_USER
	return webutil.StatusOK(c, msgPlanInfo, subscriptionpb.PlansLite_PlanLite{
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
// @Param        req         body     subscriptionpb.Plan
// @Param        key_id      path     int true "key_id"
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/subscriptions/plans/:plan_id [patch]
func (h *Handler) patchSubscriptionPlan(c *fiber.Ctx) error {
	request := new(subscriptionpb.UpdatePlan_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	request.PlanId = c.Params("plan_id")

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(subscriptionpb.UpdatePlan_RequestMultiError) {
			e := err.(subscriptionpb.UpdatePlan_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	if !userParameter.IsUserAdmin() {
		return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := subscriptionpb.NewSubscriptionHandlersClient(h.Grpc.Client)
	_, err := rClient.UpdatePlan(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgPlanUpdated, nil)
}
