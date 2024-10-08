package member

import (
	"testing"

	"github.com/werbot/werbot/api/auth"
	"github.com/werbot/werbot/internal/utils/test"
)

const (
	pathMembers              = "/v1/members"
	pathMembersProject       = pathMembers + "/project"
	pathMembersScheme        = pathMembers + "/scheme"
	pathMembersInvite        = pathMembers + "/invite"
	pathMembersInviteProject = pathMembersInvite + "/project"
)

func setupTest(t *testing.T) (*test.APIHandler, func(t *testing.T), map[string]string, map[string]string) {
	app, teardownTestCase := test.API(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	adminHeader, userHeader := app.TestUserAuth()

	return app, teardownTestCase, adminHeader, userHeader
}
