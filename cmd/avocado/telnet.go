package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal"
	auditmessage "github.com/werbot/werbot/internal/core/audit/proto/message"
	auditrpc "github.com/werbot/werbot/internal/core/audit/proto/rpc"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/core/scheme/services/ssh/auditor"
	"github.com/werbot/werbot/internal/core/scheme/services/ssh/pty"
	"github.com/werbot/werbot/internal/core/scheme/services/telnet"
)

var (
	errTelnetHostUnavailable = errors.New("telnet host unavailable")
	errTelnetSchemeType      = errors.New("scheme is not a telnet server")
)

type ptyInfo struct {
	Width  int
	Height int
	Term   string
}

func dialTelnet(ctx context.Context, schemeID, clientIP, termType string) (*telnet.Conn, []byte, error) {
	accessScheme, err := schemeAccess(ctx, schemeID, clientIP)
	if err != nil {
		return nil, nil, err
	}

	serverTelnet := accessScheme.GetServerTelnet()
	if serverTelnet == nil {
		return nil, nil, errTelnetSchemeType
	}
	if serverTelnet.GetAccess() == nil {
		return nil, nil, errors.New("telnet scheme password is missing")
	}

	address := net.JoinHostPort(serverTelnet.GetAddress(), strconv.Itoa(int(serverTelnet.GetPort())))
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", errTelnetHostUnavailable, err)
	}

	target := telnet.NewClient(conn, termType)
	output, err := telnet.Login(ctx, target, serverTelnet.GetAccess().GetLogin(), serverTelnet.GetAccess().GetPassword())
	if err != nil {
		_ = target.Close()
		return nil, nil, err
	}

	return target, output, nil
}

func connectToTelnet(scheme *schemepb.Scheme_Response, actx *authContext, newChan gossh.NewChannel, ch channelTunnel) {
	if newChan.ChannelType() != channelTypeSession {
		sendMessageInChannel(ch.lch, "Port forwarding is not supported for telnet schemes\n")
		_ = ch.lch.Close()
		return
	}

	info, ok := waitShell(ch.lreqs)
	if !ok {
		_ = ch.lch.Close()
		return
	}
	if info.Term == "" {
		info.Term = "xterm"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	target, welcome, err := dialTelnet(ctx, scheme.GetSchemeId(), actx.clientIP, info.Term)
	if err != nil {
		app.log.Error(err).Msg("Telnet connection error")
		message := telnetConnectError(err)
		sendMessageInChannel(ch.lch, message+"\n")
		_ = ch.lch.Close()
		return
	}
	defer func() { _ = target.Close() }()

	app.broker.AccountStatus(scheme.GetSchemeId(), "online")
	defer app.broker.AccountStatus(scheme.GetSchemeId(), "offline")

	log.Info().
		Str("alias", actx.alias).
		Str("clientIP", actx.clientIP).
		Str("schemeID", actx.schemeID).
		Str("userID", actx.userID).
		Msg("Open virtual connection")

	newAudit := &auditmessage.AddAudit_Request{
		ServerId:  scheme.GetSchemeId(),
		AccountId: scheme.GetSchemeMemberId(),
		ProfileId: actx.userID,
		ClientIp:  actx.clientIP,
		Session:   actx.sessionID,
	}

	recordCount := internal.GetInt32("SSHSERVER_RECORD_COUNT", 50)
	var out io.Writer = ch.lch
	auditID := ""
	if wrapped := auditor.NewLogchannel(newAudit, ch.lch, app.grpc, recordCount); wrapped != nil {
		auditID = wrapped.AuditID
		out = wrapped
		defer wrapped.Close()
	}

	setInitialWindowSize(target, info.Width, info.Height)

	_ctx, cancelAudit := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelAudit()

	rClient := auditrpc.NewAuditHandlersClient(app.grpc)
	_, err = rClient.UpdateAudit(_ctx, &auditmessage.UpdateAudit_Request{
		AuditId:   auditID,
		ProfileId: actx.userID,
		Session:   actx.sessionID,
		Width:     int32(info.Width),  //nolint:gosec // G115: pty dimensions come from a 4-byte wire field
		Height:    int32(info.Height), //nolint:gosec // G115: pty dimensions come from a 4-byte wire field
		EnvTerm:   info.Term,
	})
	if err != nil {
		app.log.Error(err).Msg("gRPC AuditUpdate")
	}

	if len(welcome) > 0 {
		if _, err := out.Write(welcome); err != nil {
			app.log.Error(err).Msg("Telnet welcome write")
		}
	}

	if err := pipeTelnet(ch.lch, ch.lreqs, target, out); err != nil {
		app.log.Error(err).Msg("Telnet pipe error")
	}

	log.Info().
		Str("alias", actx.alias).
		Str("clientIP", actx.clientIP).
		Str("schemeID", actx.schemeID).
		Str("userID", actx.userID).
		Msg("Closed virtual connection")
}

func waitShell(reqs <-chan *gossh.Request) (ptyInfo, bool) {
	var info ptyInfo

	for req := range reqs {
		switch req.Type {
		case "pty-req":
			parsed, ok := pty.ParsePtyRequest(req.Payload)
			if !ok {
				_ = req.Reply(false, nil)
				continue
			}
			info = ptyInfo{Width: parsed.Width, Height: parsed.Height, Term: parsed.EnvTerm}
			_ = req.Reply(true, nil)

		case "env":
			_ = req.Reply(true, nil)

		case "shell":
			_ = req.Reply(true, nil)
			return info, true

		case "exec", "subsystem":
			_ = req.Reply(false, nil)
			return info, false

		default:
			_ = req.Reply(false, nil)
		}
	}

	return info, false
}

func pipeTelnet(lch gossh.Channel, lreqs <-chan *gossh.Request, target *telnet.Conn, out io.Writer) error {
	defer func() { _ = lch.Close() }()
	defer func() { _ = target.Close() }()

	done := make(chan error, 2)
	go func() {
		_, err := io.Copy(out, target)
		done <- err
	}()
	go func() {
		_, err := io.Copy(target, lch)
		done <- err
	}()

	go func() {
		for req := range lreqs {
			switch req.Type {
			case "window-change":
				width, height, ok := parseWindowChange(req.Payload)
				if ok {
					if err := target.SetWindowSize(width, height); err != nil {
						ok = false
					}
				}
				_ = req.Reply(ok, nil)
			default:
				_ = req.Reply(false, nil)
			}
		}
	}()

	err := <-done
	if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func parseWindowChange(payload []byte) (width, height int, ok bool) {
	if len(payload) < 8 {
		return 0, 0, false
	}
	return int(binary.BigEndian.Uint32(payload[0:4])), int(binary.BigEndian.Uint32(payload[4:8])), true
}

func setInitialWindowSize(target *telnet.Conn, width, height int) {
	if width > 0 && height > 0 {
		_ = target.SetWindowSize(width, height)
	}
}

func telnetConnectError(err error) string {
	switch {
	case errors.Is(err, telnet.ErrLoginFailed):
		return "Telnet login failed"
	case errors.Is(err, telnet.ErrLoginTimeout):
		return "Telnet login timeout"
	case errors.Is(err, errTelnetHostUnavailable):
		return "Host unavailable"
	default:
		return err.Error()
	}
}
