package app

import (
	"context"
	"log/slog"
	"regexp"
	"runtime/debug"
	"sync"
)

var channelName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

type subscription struct {
	module string
	fn     func(context.Context, string)
}

// subscriptions collects the NOTIFY subscriptions of modules during Init.
// handlers seals it: a later add means Subscribe was called outside Init.
type subscriptions struct {
	mu     sync.Mutex
	byChan map[string][]subscription
	sealed bool
}

// add registers fn of module for channel. It panics on an invalid channel, a
// nil fn or a call after handlers.
func (s *subscriptions) add(module, channel string, fn func(context.Context, string)) {
	if !channelName.MatchString(channel) {
		panic(`sdk: invalid channel "` + channel + `"`)
	}
	if fn == nil {
		panic(`sdk: nil handler for channel "` + channel + `"`)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sealed {
		panic("sdk: Subscribe called outside Init")
	}
	if s.byChan == nil {
		s.byChan = map[string][]subscription{}
	}
	s.byChan[channel] = append(s.byChan[channel], subscription{module, fn})
}

// handlers seals s and returns one handler per channel for pgnotify.Listen:
// the core handler (when there is one), then the module functions in add
// order, each with ctx. A panic in a module function is logged and the next
// handler still runs.
func (s *subscriptions) handlers(ctx context.Context, core map[string]func(string), logger *slog.Logger) map[string]func(string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sealed = true
	out := make(map[string]func(string), len(core)+len(s.byChan))
	for c, fn := range core {
		out[c] = fn
	}
	for c, subs := range s.byChan {
		coreFn := core[c]
		out[c] = func(payload string) {
			if coreFn != nil {
				coreFn(payload)
			}
			for _, sub := range subs {
				callModule(ctx, logger, sub, c, payload)
			}
		}
	}
	return out
}

func callModule(ctx context.Context, logger *slog.Logger, sub subscription, channel, payload string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("module subscription panic", "module", sub.module, "channel", channel, "panic", r, "stack", string(debug.Stack()))
		}
	}()
	sub.fn(ctx, payload)
}
