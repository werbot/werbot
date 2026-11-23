package app

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/tunnel"
)

// tunnelMFA is identity.Service as tunnel.MFA: challenges of gateway
// instances and the age of a binding's last second factor.
type tunnelMFA struct{ ids *identity.Service }

func (m tunnelMFA) Create(ctx context.Context, r tunnel.MFAChallengeRequest) (uuid.UUID, string, string, time.Time, error) {
	c, err := m.ids.CreateMFAChallenge(ctx, identity.MFAChallengeInput{OrgID: r.OrgID, UserID: r.UserID, InstanceID: &r.InstanceID,
		ResourceID: &r.ResourceID, SSHSessionID: r.SSHSessionID, ClientIP: r.ClientIP, Binding: r.Binding, Strength: r.Strength})
	return c.ID, c.Code, c.URL, c.ExpiresAt, err
}

// VerifyTOTP answers a wrong or reused code as false; the attempt limit,
// another instance's challenge and one that is not pending stay errors.
func (m tunnelMFA) VerifyTOTP(ctx context.Context, orgID, instanceID, id uuid.UUID, code string) (bool, error) {
	err := m.ids.VerifyChallengeTOTP(ctx, orgID, instanceID, id, code)
	if errors.Is(err, identity.ErrInvalidCode) {
		return false, nil
	}
	return err == nil, err
}

func (m tunnelMFA) Status(ctx context.Context, orgID, instanceID, id uuid.UUID) (string, error) {
	return m.ids.ChallengeStatus(ctx, orgID, instanceID, id)
}

func (m tunnelMFA) Confirmed(ctx context.Context, orgID, instanceID uuid.UUID) ([]uuid.UUID, error) {
	return m.ids.ConfirmedChallenges(ctx, orgID, instanceID)
}

func (m tunnelMFA) Age(ctx context.Context, orgID, userID uuid.UUID, binding string) (int64, string, error) {
	age, method, ok, err := m.ids.MFAAge(ctx, orgID, userID, binding)
	if err != nil || !ok {
		return tunnel.NoMFA, "", err
	}
	return int64(age / time.Second), method, nil
}
