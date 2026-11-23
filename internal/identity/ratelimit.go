package identity

import (
	"context"
	"time"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/rate"
)

// Rate limits (contract §3.6): Login per client IP and per client IP and
// email each minute, password reset requests per email each hour.
const (
	LoginPerIP      = 100
	LoginPerIPEmail = 10
	ResetPerEmail   = 3
)

// limit counts one call of key before and outside any effect transaction
// (rate.Allow commits on its own). Past n calls in window it returns
// apperr.ErrResourceExhausted; a store error refuses too (fail-closed).
func (s *Service) limit(ctx context.Context, key string, n int, window time.Duration) error {
	ok, err := rate.Allow(ctx, s.pool, key, n, window)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ErrResourceExhausted
	}
	return nil
}

// limitLogin counts a login attempt of email (normalized) from the client IP.
func (s *Service) limitLogin(ctx context.Context, email string) error {
	ip := clientip.From(ctx).String()
	if err := s.limit(ctx, "login:ip:"+ip, LoginPerIP, time.Minute); err != nil {
		return err
	}
	return s.limit(ctx, "login:ipemail:"+ip+":"+email, LoginPerIPEmail, time.Minute)
}
