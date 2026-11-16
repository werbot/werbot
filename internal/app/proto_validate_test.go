package app

import (
	"testing"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/assert"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

func TestRequestValidation(t *testing.T) {
	ok := &werbotv1.SetupRequest{Email: "a@example.com", Login: "alice", Name: "Alice", Password: "correct horse battery", OrganizationName: "Acme"}
	assert.NoError(t, protovalidate.Validate(ok))
	assert.Error(t, protovalidate.Validate(&werbotv1.SetupRequest{Email: "nope", Login: "Al+ce", Name: "A", Password: "short", OrganizationName: "Acme"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.InviteMemberRequest{Email: "a@example.com"})) // ROLE_UNSPECIFIED
	assert.NoError(t, protovalidate.Validate(&werbotv1.AcceptInviteRequest{Token: "t"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.AcceptInviteRequest{Token: "t", Login: "UPPER"}))
}
