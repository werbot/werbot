package scheme_test

import (
	"testing"

	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/pkg/utils/errutil"
	"github.com/werbot/werbot/pkg/utils/protoutils"
)

func Test_DbgActivityValidation(t *testing.T) {
	bad := make([]int32, 23)
	for i := range bad {
		bad[i] = 2
	}
	good := make([]int32, 23)

	req := &schemepb.UpdateSchemeActivity_Request{
		OwnerId:   "008feb1d-12f2-4bc3-97ff-c8d7fb9f7686",
		ProjectId: "2bef1080-cd6e-49e5-8042-1224cf6a3da9",
		SchemeId:  "156d8d65-cfe5-48a4-a636-198a5f509abf",
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
