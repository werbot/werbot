//go:build linux || darwin

package vnet

import (
	"github.com/google/uuid"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// resourcesOf keeps the resources the caller may connect to now that have a
// stream or datagram kind: only those get an address.
func resourcesOf(list []*werbotv1.AccessibleResource) []Resource {
	var out []Resource
	for _, a := range list {
		id, err := uuid.Parse(a.GetId())
		r := Resource{ID: id, Name: a.GetName(), Kinds: a.GetKinds(), SSHAddress: a.GetSshAddress(), WSURL: a.GetWsUrl(), HostKey: a.GetHostKey()}
		if err == nil && a.GetAllowed() && protos(r) != nil {
			out = append(out, r)
		}
	}
	return out
}
