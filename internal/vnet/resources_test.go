//go:build linux || darwin

package vnet

import (
	"testing"

	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

func TestResourcesOfAllowedPortsOnly(t *testing.T) {
	pg := res("pg", "stream")
	list := []*werbotv1.AccessibleResource{
		{Id: pg.ID.String(), Name: "pg", Kinds: []string{"stream"}, Allowed: true, SshAddress: "gw:2222", WsUrl: "wss://gw", HostKey: "ssh-ed25519 AAAA"},
		{Id: res("dns", "datagram").ID.String(), Name: "dns", Kinds: []string{"datagram"}, Allowed: false, Requestable: true},
		{Id: res("web", "shell").ID.String(), Name: "web", Kinds: []string{"shell", "exec"}, Allowed: true},
		{Id: "not-a-uuid", Name: "bad", Kinds: []string{"stream"}, Allowed: true},
	}
	pg.SSHAddress, pg.WSURL, pg.HostKey = "gw:2222", "wss://gw", "ssh-ed25519 AAAA"
	require.Equal(t, []Resource{pg}, resourcesOf(list))
}
