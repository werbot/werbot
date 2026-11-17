package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/sshdgram"
)

// resourceInfo answers resource@werbot.com for a direct login with the type,
// kinds and ports of the resource, after the check Open makes with port 0. A
// refusal, an unknown resource and a menu login get false without payload: the
// answer tells nothing about resources the user cannot open. A gateway that is
// not ready says so, as on every channel: that is the state of the instance, not
// of a resource. Requests of a connection run one at a time, so the control
// plane lookup is bounded.
func (h *sshHandler) resourceInfo(ctx ssh.Context, _ *ssh.Server, _ *gossh.Request) (bool, []byte) {
	_, resource, _ := ParseLogin(ctx.User()) // checked at the handshake
	notReady := []byte(openText(ErrNotReady, resource, "", nil))
	switch {
	case resource == "":
		return false, nil
	case handshakeUser(ctx) == nil:
		return false, notReady
	}
	dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	r := h.request(ctx, resource, "", Window{}, "")
	_, req, _, err := h.p.decide(dctx, r)
	if err != nil {
		h.p.deny(r, req.User.ID, err)
		if errors.Is(err, ErrNotReady) {
			return false, notReady
		}
		return false, nil
	}
	b, err := json.Marshal(info(req.Resource))
	return err == nil, b
}

// info lists the type's kinds and the endpoint's ports; a type without an
// endpoint, or one allowing any target without ports, has none.
func info(res Resource) sshdgram.ResourceInfo {
	typ, _ := connector.Lookup(res.Type) // the PDP allowed it: the type is registered
	out := sshdgram.ResourceInfo{Type: res.Type, Kinds: make([]string, 0, len(typ.Kinds)), Ports: []sshdgram.Port{}}
	for _, k := range typ.Kinds {
		out.Kinds = append(out.Kinds, string(k))
	}
	if ep, ok, _ := res.Endpoint(); ok {
		for _, p := range ep.Ports {
			out.Ports = append(out.Ports, sshdgram.Port{Port: p.Port, EndPort: p.EndPort})
		}
	}
	return out
}
