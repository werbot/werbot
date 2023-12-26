package ghost

import "github.com/werbot/werbot/pkg/logger"

var log logger.Logger

func init() {
  log = logger.New()
}
