package token

/*
import (
	"net/http"
	"testing"

	"github.com/werbot/werbot/internal/utils/test"
)

// TODO 1. registration if there is no profile, 2. authorization if the profile is not authorized,
func TestHandler_memberInviteActivate(t *testing.T) {
	app, teardownTestCase, _, userHeader := setupTest(t)
	defer teardownTestCase(t)

	testTable := []test.APITable{
		{ // fake invite
			Name:       "test0_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathInvitesMember, test.ConstFakeID),
			StatusCode: 404,
			Body:       test.BodyNotFound,
		},
		{ // USER: request
			Name:       "test2_01",
			Method:     http.MethodGet,
			Path:       test.PathGluing(pathInvitesMember, test.ConstUserMemberInviteID),
			StatusCode: 200,
			Body: test.BodyTable{
				"code":    float64(200),
				"message": "Invite confirmed",
			},
			RequestHeaders: userHeader,
		},
		// TODO add other test cases to activate member invite
	}

	test.RunCaseAPITests(t, app, testTable)
}
*/
