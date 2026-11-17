package connectcli

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"

	"github.com/werbot/werbot/internal/platform/sshdgram"
)

// DefaultLocalPort is the local port for a target port: the same, or +10000
// below 1024, which needs no privileges.
func DefaultLocalPort(target uint16) uint16 {
	if target < 1024 {
		return target + 10000
	}
	return target
}

// targetPort is the port given in sp or the only port of the resource.
func targetPort(sp Spec, ports []sshdgram.Port) (uint16, error) {
	switch {
	case sp.TargetPort != 0:
		return sp.TargetPort, nil
	case len(ports) == 0:
		return 0, fmt.Errorf("resource %s has no ports; use %s:<port>", sp.Resource, sp.Resource)
	case len(ports) == 1 && (ports[0].EndPort == 0 || ports[0].EndPort == ports[0].Port):
		return ports[0].Port, nil
	}
	list := make([]string, len(ports))
	for i, p := range ports {
		list[i] = strconv.Itoa(int(p.Port))
		if p.EndPort > p.Port {
			list[i] += "-" + strconv.Itoa(int(p.EndPort))
		}
	}
	return 0, fmt.Errorf("resource %s has several ports (%s); use %s:<port>", sp.Resource, strings.Join(list, ", "), sp.Resource)
}

// wsaeaddrinuse is WSAEADDRINUSE: Windows reports a busy port with it, not EADDRINUSE.
const wsaeaddrinuse = syscall.Errno(10048)

// listen opens the local port of resource on 127.0.0.1 only: a TCP listener,
// or with udp a packet socket. Every local listener of werbot connect is
// created here. A busy port fails with a free one to use instead.
func listen(udp bool, port uint16, resource string) (net.Listener, net.PacketConn, error) {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
	var ln net.Listener
	var pc net.PacketConn
	var err error
	if udp {
		pc, err = net.ListenPacket("udp", addr)
	} else {
		ln, err = net.Listen("tcp", addr)
	}
	if errors.Is(err, syscall.EADDRINUSE) || errors.Is(err, wsaeaddrinuse) {
		return nil, nil, fmt.Errorf("local port %d is busy; use %s:%s", port, freePort(udp), resource)
	}
	return ln, pc, err
}

// freePort is a port the OS picks as free now, or "<port>" if it cannot.
func freePort(udp bool) string {
	ln, pc, err := listen(udp, 0, "")
	if err != nil {
		return "<port>"
	}
	var a net.Addr
	if udp {
		a = pc.LocalAddr()
		_ = pc.Close()
	} else {
		a = ln.Addr()
		_ = ln.Close()
	}
	_, port, _ := net.SplitHostPort(a.String())
	return port
}
