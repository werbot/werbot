package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/principal"
)

// FreshMFAWindow is how recent the caller's second factor must be for the
// procedures RequireFreshMFA guards.
const FreshMFAWindow = 5 * time.Minute

var (
	errMFARequired        = errors.New("mfa required")
	errEnrollmentRequired = errors.New("mfa enrollment required")
)

// enrollmentServices stay open while the caller must enroll a factor, with
// enrollmentProcedure.
var enrollmentServices = []string{
	"/" + werbotv1connect.AuthServiceName + "/",
	"/" + werbotv1connect.MFAServiceName + "/",
	"/" + werbotv1connect.SystemServiceName + "/",
}

const enrollmentProcedure = werbotv1connect.ProfileServiceGetProfileProcedure

// factorProcedures change the caller's own factors: they need a fresh second
// factor once the user has one (the first enrollment passes).
var factorProcedures = map[string]bool{
	werbotv1connect.MFAServiceEnrollTOTPProcedure:                true,
	werbotv1connect.MFAServiceDeleteFactorProcedure:              true,
	werbotv1connect.MFAServiceGenerateRecoveryCodesProcedure:     true,
	werbotv1connect.MFAServiceBeginWebAuthnRegistrationProcedure: true,
}

// freshMFA is the RequireFreshMFA interceptor.
type freshMFA struct{ procedures map[string]bool }

// RequireFreshMFA gates Connect procedures on the caller's MFA state that
// Middleware put into the context (no database call). A caller who must
// enroll a factor (EnrollmentRequired) reaches only AuthService,
// MFAService, SystemService and ProfileService/GetProfile; others get
// PermissionDenied "mfa enrollment required". procedures, a break-glass
// access request and, once the user has a factor, the factor changes of
// MFAService need a second factor of the credential within FreshMFAWindow
// when the user has a factor or require_mfa asks one of the caller's role;
// an agent token never has one. Otherwise: PermissionDenied "mfa required".
// It runs after RestrictAgentTokens, whose answers come first.
func RequireFreshMFA(procedures map[string]bool) connect.Interceptor {
	return freshMFA{procedures: procedures}
}

func (f freshMFA) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		cr, _ := req.Any().(*werbotv1.CreateRequestRequest)
		if err := f.admit(ctx, req.Spec().Procedure, cr.GetBreakGlass()); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (freshMFA) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (f freshMFA) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := f.admit(ctx, conn.Spec().Procedure, false); err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

// admit applies the enrollment gate, then the freshness check, to procedure.
func (f freshMFA) admit(ctx context.Context, procedure string, breakGlass bool) error {
	p, ok := principal.From(ctx)
	if !ok {
		return nil // public procedures; the others answer Unauthenticated themselves
	}
	if EnrollmentRequired(ctx) && !enrollmentOpen(procedure) {
		return connect.NewError(connect.CodePermissionDenied, errEnrollmentRequired)
	}
	st, _ := ctx.Value(mfaStateKey{}).(mfaState)
	if f.needsFresh(procedure, breakGlass, p, st) && !st.fresh(p) {
		return connect.NewError(connect.CodePermissionDenied, errMFARequired)
	}
	return nil
}

func enrollmentOpen(procedure string) bool {
	if procedure == enrollmentProcedure {
		return true
	}
	for _, svc := range enrollmentServices {
		if strings.HasPrefix(procedure, svc) {
			return true
		}
	}
	return false
}

func (f freshMFA) needsFresh(procedure string, breakGlass bool, p principal.Principal, st mfaState) bool {
	if factorProcedures[procedure] {
		return st.HasFactor
	}
	return (f.procedures[procedure] || breakGlass) && (st.HasFactor || mfaRequired(st.RequireMFA, p))
}

// fresh reports a second factor of the credential within FreshMFAWindow;
// an agent token never has one.
func (st mfaState) fresh(p principal.Principal) bool {
	return !p.IsAgent() && st.FreshAt != nil && time.Since(*st.FreshAt) <= FreshMFAWindow
}
