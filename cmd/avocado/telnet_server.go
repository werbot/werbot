package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/armon/go-proxyproto"

	"github.com/werbot/werbot/internal"
	auditmessage "github.com/werbot/werbot/internal/core/audit/proto/message"
	auditrpc "github.com/werbot/werbot/internal/core/audit/proto/rpc"
	firewallmessage "github.com/werbot/werbot/internal/core/firewall/proto/message"
	firewallrpc "github.com/werbot/werbot/internal/core/firewall/proto/rpc"
	"github.com/werbot/werbot/internal/core/scheme/services/ssh/auditor"
	"github.com/werbot/werbot/internal/core/scheme/services/telnet"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenrpc "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/version"
	"github.com/werbot/werbot/pkg/utils/netutil"
	"github.com/werbot/werbot/pkg/uuid"
)

const telnetTokenAttempts = 3

// serveTelnet accepts plaintext telnet connections for one-time token auth.
func serveTelnet(addr string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		app.log.Fatal(err).Msg("Start telnet server")
		return
	}
	defer listener.Close()

	app.log.Info().Str("serverAddress", addr).Msg("Telnet Server accepting connections")

	proxyListener := &proxyproto.Listener{Listener: listener}
	for {
		conn, err := proxyListener.Accept()
		if err != nil {
			app.log.Error(err).Msg("Telnet accept")
			return
		}
		go handleTelnetConn(conn)
	}
}

func handleTelnetConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	clientIP := remoteIP(conn.RemoteAddr())

	firewallCtx, cancelFirewall := context.WithTimeout(context.Background(), 10*time.Second)
	_, err := firewallrpc.NewFirewallHandlersClient(app.grpc).IPAccess(firewallCtx, &firewallmessage.IPAccess_Request{
		ClientIp: clientIP,
	})
	cancelFirewall()
	if err != nil {
		_, _ = conn.Write([]byte("Access denied\r\n"))
		return
	}

	if timeout := time.Duration(internal.GetInt("SSHSERVER_IDLE_TIMEOUT", 300)) * time.Second; timeout > 0 {
		conn = &idleConn{Conn: conn, timeout: timeout}
	}

	telnetConn, err := telnet.NewServer(conn)
	if err != nil {
		app.log.Error(err).Msg("Telnet negotiation")
		return
	}
	defer func() { _ = telnetConn.Close() }()

	writeTelnetText(telnetConn, " _    _  ____  ____  ____  _____  ____ \n( \\/\\/ )( ___)(  _ \\(  _ \\(  _  )(_  _)\n \033[0;31m)    (  )__)  )   / ) _ < )(_)(   )(\033[0m  \n(__/\\__)(____)(_)\\_)(____/(_____) (__) \n"+version.Version()+", "+version.Commit()+", "+version.BuildDate()+"\n\n")

	tokenResponse, err := readToken(telnetConn, telnetTokenAttempts, func(token string) (*tokenmessage.UseTokenSchemeConnect_Response, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return tokenrpc.NewTokenHandlersClient(app.grpc).UseTokenSchemeConnect(ctx, &tokenmessage.UseTokenSchemeConnect_Request{
			Token: token,
		})
	})
	if err != nil {
		app.log.Info().Err(err).Str("clientIP", clientIP).Msg("Telnet token rejected")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	accessScheme, err := schemeAccess(ctx, tokenResponse.GetSchemeId(), clientIP)
	if err != nil {
		writeTelnetText(telnetConn, err.Error()+"\n")
		return
	}
	if accessScheme.GetServerTelnet() == nil {
		writeTelnetText(telnetConn, "Only telnet schemes are available on this port\n")
		return
	}

	termType := telnetConn.TermType()
	if termType == "" {
		termType = "xterm"
	}

	target, welcome, err := dialTelnet(ctx, tokenResponse.GetSchemeId(), clientIP, termType)
	if err != nil {
		app.log.Error(err).Msg("Telnet listener connection error")
		writeTelnetText(telnetConn, telnetConnectError(err)+"\n")
		return
	}
	defer func() { _ = target.Close() }()

	if width, height := telnetConn.Size(); width > 0 && height > 0 {
		setInitialWindowSize(target, width, height)
	}
	telnetConn.OnResize = func(width, height int) {
		_ = target.SetWindowSize(width, height)
	}

	app.broker.AccountStatus(tokenResponse.GetSchemeId(), "online")
	defer app.broker.AccountStatus(tokenResponse.GetSchemeId(), "offline")

	sessionID := uuid.New()
	newAudit := &auditmessage.AddAudit_Request{
		ServerId:  tokenResponse.GetSchemeId(),
		AccountId: tokenResponse.GetSchemeMemberId(),
		ProfileId: tokenResponse.GetProfileId(),
		ClientIp:  clientIP,
		Session:   sessionID,
	}

	recordCount := internal.GetInt32("SSHSERVER_RECORD_COUNT", 50)
	var out io.Writer = telnetConn
	auditID := ""
	if wrapped := auditor.NewLogchannel(newAudit, telnetConn, app.grpc, recordCount); wrapped != nil {
		auditID = wrapped.AuditID
		out = wrapped
		defer wrapped.Close()
	}

	auditCtx, cancelAudit := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelAudit()
	width, height := telnetConn.Size()
	if _, err := auditrpc.NewAuditHandlersClient(app.grpc).UpdateAudit(auditCtx, &auditmessage.UpdateAudit_Request{
		AuditId:   auditID,
		ProfileId: tokenResponse.GetProfileId(),
		Session:   sessionID,
		Width:     int32(width),  //nolint:gosec // G115: NAWS dimensions are 16-bit
		Height:    int32(height), //nolint:gosec // G115: NAWS dimensions are 16-bit
		EnvTerm:   termType,
	}); err != nil {
		app.log.Error(err).Msg("gRPC AuditUpdate")
	}

	if len(welcome) > 0 {
		if _, err := out.Write(welcome); err != nil {
			app.log.Error(err).Msg("Telnet welcome write")
		}
	}

	pipeTelnetListener(telnetConn, target, out)
}

//nolint:unparam // attempts is part of the listener auth contract and is passed explicitly by callers.
func readToken(c *telnet.Conn, attempts int, use func(token string) (*tokenmessage.UseTokenSchemeConnect_Response, error)) (*tokenmessage.UseTokenSchemeConnect_Response, error) {
	var pendingLF bool

	for attempt := 0; attempt < attempts; attempt++ {
		if _, err := c.Write([]byte("Token: ")); err != nil {
			return nil, err
		}

		token, err := readTokenLine(c, &pendingLF)
		if err != nil {
			return nil, err
		}

		response, err := use(token)
		if err == nil {
			return response, nil
		}

		if _, writeErr := c.Write([]byte("Invalid token\r\n")); writeErr != nil {
			return nil, writeErr
		}
	}

	return nil, errors.New("invalid token")
}

func readTokenLine(c *telnet.Conn, pendingLF *bool) (string, error) {
	line := make([]byte, 0, 64)
	buf := make([]byte, 1)

	for {
		n, err := c.Read(buf)
		if n > 0 {
			b := buf[0]

			if *pendingLF {
				*pendingLF = false
				if b == '\n' {
					continue
				}
			}

			switch b {
			case '\r':
				*pendingLF = true
				return string(line), nil
			case '\n':
				return string(line), nil
			case 0:
				// CR NUL and telnet padding.
				continue
			default:
				if len(line) < 64 {
					line = append(line, b)
				}
			}
		}

		if err != nil {
			return "", err
		}
	}
}

func pipeTelnetListener(client, target *telnet.Conn, out io.Writer) {
	done := make(chan struct{}, 2)

	go func() {
		_, _ = io.Copy(out, target)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(target, client)
		done <- struct{}{}
	}()

	<-done
}

func writeTelnetText(c *telnet.Conn, text string) {
	p := bytes.ReplaceAll([]byte(text), []byte("\n"), []byte("\r\n"))
	p = bytes.ReplaceAll(p, []byte("\r\r\n"), []byte("\r\n"))
	if _, err := c.Write(p); err != nil {
		app.log.Error(err).Msg("Telnet write")
	}
}

func remoteIP(addr net.Addr) string {
	if addr == nil {
		return ""
	}

	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		host = addr.String()
	}
	return netutil.IP(host)
}

// idleConn extends the read deadline after each successful read.
type idleConn struct {
	net.Conn
	timeout time.Duration
}

func (c *idleConn) Read(p []byte) (int, error) {
	if c.timeout > 0 {
		_ = c.SetReadDeadline(time.Now().Add(c.timeout))
	}
	return c.Conn.Read(p)
}

func (c *idleConn) Write(p []byte) (int, error) {
	if c.timeout > 0 {
		_ = c.SetReadDeadline(time.Now().Add(c.timeout))
	}
	return c.Conn.Write(p)
}
