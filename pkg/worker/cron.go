package worker

import (
	"context"

	"github.com/adhocore/gronx"
)

type Cronjob struct {
	Identifier string
	Spec       CronSpec
	Handler    CronHandler
}

type CronHandler func(ctx context.Context) error

type CronSpec string

func (cs CronSpec) String() string {
	return string(cs)
}

func (cs CronSpec) Validate() bool {
	return gronx.New().IsValid(cs.String())
}

func (cs CronSpec) MustValidate() {
	if !cs.Validate() {
		panic("invalid cron specification: " + cs)
	}
}
