package notify

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/config"
)

func TestSMTPMailerTLSPolicy(t *testing.T) {
	cfg := config.SMTP{Host: "localhost", Port: 1025, From: "werbot@example.com"}
	for name, c := range map[string]struct {
		development bool
		want        string
	}{
		"production requires STARTTLS":      {false, "TLSMandatory"},
		"development allows a plain server": {true, "TLSOpportunistic"},
	} {
		m, err := NewSMTPMailer(cfg, c.development)
		require.NoError(t, err, name)
		require.Equal(t, c.want, m.client.TLSPolicy(), name)
	}
}
