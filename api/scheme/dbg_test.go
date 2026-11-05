package scheme_test

import (
	"testing"

	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/pkg/utils/errutil"
	"github.com/werbot/werbot/pkg/utils/protoutils"
)

func Test_DbgActivityValidation(t *testing.T) {
	bad := make([]int32, 24)
	for i := range bad {
		bad[i] = 2
	}
	good := make([]int32, 24)

	req := &schemepb.UpdateSchemeActivity_Request{
		OwnerId:   "11111111-1111-1111-1111-111111111111",
		ProjectId: "22222222-2222-2222-2222-222222222222",
		SchemeId:  "33333333-3333-3333-3333-333333333333",
		Activity: &schemepb.SchemeActivity_Week{
			Mon: bad,
			Tue: good,
			Wed: good,
			Thu: good,
			Fri: good,
			Sat: good,
			Sun: good,
		},
	}

	err := protoutils.ValidateRequest(req)
	t.Logf("err = %v", err)

	if em, ok := err.(errutil.ErrorMap); ok {
		for k, v := range em {
			t.Logf("key=%q val=%q", k, v)
		}
	}
}
