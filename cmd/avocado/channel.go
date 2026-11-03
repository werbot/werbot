package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/olekukonko/tablewriter"
	tw "github.com/olekukonko/tablewriter/tw"
	"github.com/rs/zerolog/log"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"github.com/werbot/werbot/internal"
	auditmessage "github.com/werbot/werbot/internal/core/audit/proto/message"
	auditrpc "github.com/werbot/werbot/internal/core/audit/proto/rpc"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/core/scheme/services/ssh/auditor"
	"github.com/werbot/werbot/internal/core/scheme/services/ssh/pty"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/version"
	"github.com/werbot/werbot/pkg/utils/strutil"
	"github.com/werbot/werbot/pkg/uuid"
)

type sessionConfig struct {
	ClientConfig *gossh.ClientConfig
	Channel      channelTunnel

	Addr      string
	AccountID string
	ClientIP  string
	SessionID string
}

type channelTunnel struct {
	lch   gossh.Channel
	lreqs <-chan *gossh.Request
	err   error
}

func connectToHost(scheme *schemepb.Scheme_Response, actx *authContext, sshCtx ssh.Context, newChan gossh.NewChannel, srv *ssh.Server, conn *gossh.ServerConn, ch channelTunnel) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := newSession(ctx, scheme, actx)
	if err != nil {
		app.log.Error(err).Msg("Bastion ClientConfig error")
		actx.message = fmt.Sprintf("%v", err)
		sendMessageInChannel(ch.lch, actx.message+"\n")
		_ = ch.lch.Close()
		return
	}

	app.broker.AccountStatus(scheme.GetSchemeId(), "online")

	sessionConfigs := []sessionConfig{{
		Addr:         client.Address,
		ClientConfig: client.ClientConfig,
		AccountID:    scheme.GetSchemeId(),
		ClientIP:     actx.clientIP,
		SessionID:    actx.sessionID,
		Channel:      ch,
	}}

	log.Info().
		Str("alias", actx.alias).
		Str("clientIP", actx.clientIP).
		Str("schemeID", actx.schemeID).
		Str("userID", actx.userID).
		Msg("Open virtual connection")

	go func() {
		if err := multiChannelHandler(srv, conn, newChan, sshCtx, sessionConfigs); err != nil {
			app.log.Error(err).Msg("Multi ChannelHandler error")
			conn.Close()
		}

		app.broker.AccountStatus(scheme.GetSchemeId(), "offline")

		actx.message = "Host unavailable"
		log.Info().
			Str("alias", actx.alias).
			Str("clientIP", actx.clientIP).
			Str("schemeID", actx.schemeID).
			Str("userID", actx.userID).
			Msg("Closed virtual connection")
	}()
}

func channelHandler(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
	switch newChan.ChannelType() {
	case channelTypeSession:
	case channelTypeDirectTCPIP:
	default:
		if err := newChan.Reject(gossh.UnknownChannelType, "unsupported channel type"); err != nil {
			app.log.Error(err).Msg("Failed to reject channel")
		}
		return
	}

	actx := ctx.Value(authContextKey).(*authContext)

	ch, req, err := newChan.Accept()
	if err != nil {
		app.log.Error(err).Msg("Could not accept channel")
		return
	}

	_ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := schemepb.NewSchemeHandlersClient(app.grpc)

	switch actx.userType() {
	case schemepb.Type_healthcheck:
		sendMessageInChannel(ch, "OK\n")
		_ = ch.Close()
		return

	case schemepb.Type_invite:
		app.log.Info().Str("invite", actx.message).Str("clientIP", actx.clientIP).Msg(trace.MsgTokenIsInvalid)
		sendMessageInChannel(ch, fmt.Sprintf("Invite %s is invalid.\n", actx.message))
		_ = ch.Close()
		return

	case schemepb.Type_shell:
		if actx.userID == "" {
			app.log.Info().Str("alias", actx.alias).Str("clientIP", actx.clientIP).Msg(trace.MsgAccessIsDeniedUser)
			actx.message = "Firewall denied access"
			sendMessageInChannel(ch, actx.message+"\n")
			_ = ch.Close()
			return
		}

		actx.sessionID = uuid.New()

		sendMessageInChannel(ch, " _    _  ____  ____  ____  _____  ____ \n( \\/\\/ )( ___)(  _ \\(  _ \\(  _  )(_  _)\n \033[0;31m)    (  )__)  )   / ) _ < )(_)(   )(\033[0m  \n(__/\\__)(____)(_)\\_)(____/(_____) (__) \n"+version.Version()+", "+version.Commit()+", "+version.BuildDate()+"\n\n")

		schemeList, err := rClient.SystemSchemesByAlias(_ctx, &schemepb.SystemSchemesByAlias_Request{
			Alias: actx.alias,
		})
		if err != nil {
			app.log.Error(err).Str("alias", actx.alias).Msg("Schemes not found")
		}
		actx.serverList = schemeList.GetSchemes()

		if len(actx.serverList) > 0 {
			app.log.Info().Str("alias", actx.alias).Str("clientIP", actx.clientIP).Str("userID", actx.userID).Msg("Open shell console connection")

			loginArray := strutil.SplitNTrimmed(actx.alias, "_", 3)
			status := map[bool]string{
				false: "\x1B[01;31m•\x1B[0m",
				true:  "\x1B[01;32m•\x1B[0m",
			}

			message := "Hello " + loginArray[0] + ", you access to next schemes:\n"
			bufMsg := &strings.Builder{}
			table := tablewriter.NewTable(bufMsg,
				tablewriter.WithHeader([]string{"⚡", "Name", "Login for direct access", "🚚"}),
				tablewriter.WithRowAutoWrap(tw.WrapNone),
			)

			for i := 0; i < len(actx.serverList); i++ {
				scheme := actx.serverList[i]
				table.Append([]string{
					fmt.Sprintf("%v %v", status[scheme.GetOnline()], i+1),
					scheme.GetTitle(),
					scheme.GetAlias(),
					scheme.GetSchemeType().String(),
				})
			}
			table.Render()
			message += bufMsg.String()
			sendMessageInChannel(ch, message)

			term := term.NewTerminal(ch, "Select scheme or push enter to exit > ")
			selected, _ := term.ReadLine()
			selectedIndex := int(strutil.ToInt32(selected))

			switch {
			case selectedIndex <= len(actx.serverList) && selectedIndex > 0:
				selectedIndex--
				scheme := actx.serverList[selectedIndex]
				actx.schemeID = scheme.GetSchemeId()
				sendMessageInChannel(ch, "Connect to scheme-name: "+scheme.GetTitle()+"\n")
				connectToHost(scheme, actx, ctx, newChan, srv, conn, channelTunnel{ch, req, err})

			case selectedIndex > len(actx.serverList):
				sendMessageInChannel(ch, "\nScheme was not found\nBye-Bye! :(\n\n")
				_ = ch.Close()

			default:
				sendMessageInChannel(ch, "\nYou have not selected scheme\nBye-Bye! :(\n\n")
				_ = ch.Close()
			}
		} else {
			sendMessageInChannel(ch, "\nYou don't have active schemes\nBye-Bye! :(\n\n")
			_ = ch.Close()
		}
		return

	case schemepb.Type_bastion:
		if actx.userID == "" {
			app.log.Info().Str("alias", actx.alias).Str("clientIP", actx.clientIP).Msg("Permission denied")
			actx.message = "Permission denied"
			sendMessageInChannel(ch, actx.message+"\n")
			_ = ch.Close()
			return
		}

		actx.sessionID = uuid.New()

		getSchemes, err := rClient.SystemSchemesByAlias(_ctx, &schemepb.SystemSchemesByAlias_Request{
			Alias: actx.alias,
		})
		if err != nil {
			app.log.Error(err).Str("alias", actx.alias).Str("clientIP", actx.clientIP).Msg("Scheme not found (channel)")
			actx.message = "Scheme not found"
			sendMessageInChannel(ch, actx.message+"\n")
			_ = ch.Close()
			return
		}
		scheme := getSchemes.GetSchemes()[0]

		actx.schemeID = scheme.GetSchemeId()
		connectToHost(scheme, actx, ctx, newChan, srv, conn, channelTunnel{ch, req, err})
		return

	default:
		app.defaultChannelHandler(srv, conn, newChan, ctx)
	}

	_ = ch.Close()
}

func multiChannelHandler(_ *ssh.Server, _ *gossh.ServerConn, newChan gossh.NewChannel, _ ssh.Context, configs []sessionConfig) error {
	var lastClient *gossh.Client

	switch newChan.ChannelType() {
	case channelTypeSession:
		lch, lreqs, err := configs[0].Channel.lch, configs[0].Channel.lreqs, configs[0].Channel.err
		if err != nil {
			return errors.New("duplicate response received for channel")
		}

		for _, config := range configs {
			var client *gossh.Client
			if lastClient == nil {
				client, err = gossh.Dial("tcp", config.Addr, config.ClientConfig)
			} else {
				rconn, err := lastClient.Dial("tcp", config.Addr)
				if err != nil {
					return err
				}
				ncc, chans, reqs, err := gossh.NewClientConn(rconn, config.Addr, config.ClientConfig)
				if err != nil {
					return err
				}
				client = gossh.NewClient(ncc, chans, reqs)
			}
			if err != nil {
				lch.Close()
				return err
			}
			defer func() { _ = client.Close() }()
			lastClient = client
		}

		rch, rreqs, err := lastClient.OpenChannel(channelTypeSession, []byte{})
		if err != nil {
			return err
		}
		return pipe(lreqs, rreqs, lch, rch, newChan, configs[0])

	case channelTypeDirectTCPIP:
		lch, lreqs, err := configs[0].Channel.lch, configs[0].Channel.lreqs, configs[0].Channel.err
		if err != nil {
			return errors.New("duplicate response received for channel")
		}

		for _, config := range configs {
			var client *gossh.Client
			if lastClient == nil {
				client, err = gossh.Dial("tcp", config.Addr, config.ClientConfig)
			} else {
				rconn, err := lastClient.Dial("tcp", config.Addr)
				if err != nil {
					return err
				}
				ncc, chans, reqs, err := gossh.NewClientConn(rconn, config.Addr, config.ClientConfig)
				if err != nil {
					return err
				}
				client = gossh.NewClient(ncc, chans, reqs)
			}
			if err != nil {
				lch.Close()
				return err
			}
			defer func() { _ = client.Close() }()
			lastClient = client
		}

		rch, rreqs, err := lastClient.OpenChannel(channelTypeDirectTCPIP, newChan.ExtraData())
		if err != nil {
			return err
		}
		return pipe(lreqs, rreqs, lch, rch, newChan, configs[0])

	default:
		if err := newChan.Reject(gossh.UnknownChannelType, "unsupported channel type"); err != nil {
			app.log.Error(err).Msg("failed to reject chan")
		}
		return nil
	}
}

func pipe(lreqs, rreqs <-chan *gossh.Request, lch, rch gossh.Channel, newChan gossh.NewChannel, sessConfig sessionConfig) error {
	defer func() {
		_ = lch.Close()
		_ = rch.Close()
	}()

	errch := make(chan error, 1)
	quit := make(chan string, 1)

	newAudit := &auditmessage.AddAudit_Request{
		AccountId: sessConfig.AccountID,
		ClientIp:  sessConfig.ClientIP,
		Session:   sessConfig.SessionID,
	}

	recordCount := internal.GetInt32("SSHSERVER_RECORD_COUNT", 50)
	auditID := ""
	var logChannel io.Writer = lch
	if wrapped := auditor.NewLogchannel(newAudit, lch, app.grpc, recordCount); wrapped != nil {
		auditID = wrapped.AuditID
		logChannel = wrapped
		defer wrapped.Close()
	}

	channeltype := newChan.ChannelType()
	if channeltype == channelTypeSession {
		go func(quit chan string) {
			_, _ = io.Copy(logChannel, rch)
			quit <- "rch"
		}(quit)

		go func(quit chan string) {
			_, _ = io.Copy(rch, lch)
			quit <- "lch"
		}(quit)
	}

	if channeltype == channelTypeDirectTCPIP {
		d := auditor.ForwardData{}
		if err := gossh.Unmarshal(newChan.ExtraData(), &d); err != nil {
			return err
		}

		// Tunnel payload is a raw byte stream. The recording sink must be
		// separate from the client channel; writing the log back into lch
		// echoed target traffic to the client.
		wrappedlch := auditor.NewLogtunnel(lch, io.Discard, d.SourceHost)
		wrappedrch := auditor.NewLogtunnel(rch, io.Discard, d.DestinationHost)
		go func(quit chan string) {
			_, _ = io.Copy(wrappedlch, rch)
			quit <- "rch"
		}(quit)

		go func(quit chan string) {
			_, _ = io.Copy(wrappedrch, lch)
			quit <- "lch"
		}(quit)

		defer wrappedlch.Close()
		defer wrappedrch.Close()
	}

	_ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := auditrpc.NewAuditHandlersClient(app.grpc)

	go func(quit chan string) {
		for req := range lreqs {
			b, err := rch.SendRequest(req.Type, req.WantReply, req.Payload)
			if req.Type == "exec" {
				if wrapped := auditor.NewLogchannel(newAudit, lch, app.grpc, recordCount); wrapped != nil {
					command := append(slices.Clone(req.Payload), '\n')
					if _, err := wrapped.Write(command); err != nil {
						app.log.Error(err).Msg("failed to write log")
					}
					_ = wrapped.Close()
				}
			}

			if req.Type == "pty-req" {
				ptyReq, _ := pty.ParsePtyRequest(req.Payload)

				_, err = rClient.UpdateAudit(_ctx, &auditmessage.UpdateAudit_Request{
					AuditId: auditID,
					Width:   int32(ptyReq.Width),  //nolint:gosec // G115: pty dimensions come from a 4-byte wire field
					Height:  int32(ptyReq.Height), //nolint:gosec // G115: pty dimensions come from a 4-byte wire field
					EnvTerm: ptyReq.EnvTerm,
				})
				if err != nil {
					app.log.Error(err).Msg("gRPC AuditUpdate")
				}
			}

			if err != nil {
				errch <- err
			}
			if err2 := req.Reply(b, nil); err2 != nil {
				errch <- err2
			}
		}
		quit <- "lreqs"
	}(quit)

	go func(quit chan string) {
		for req := range rreqs {
			b, err := lch.SendRequest(req.Type, req.WantReply, req.Payload)
			if err != nil {
				errch <- err
			}
			if err2 := req.Reply(b, nil); err2 != nil {
				errch <- err2
			}
		}
		quit <- "rreqs"
	}(quit)

	lchEOF, rchEOF, lchClosed, rchClosed := false, false, false, false
	for {
		select {
		case err := <-errch:
			return err
		case q := <-quit:
			switch q {
			case "lch":
				lchEOF = true
				_ = rch.CloseWrite()
			case "rch":
				rchEOF = true
				_ = lch.CloseWrite()
			case "lreqs":
				lchClosed = true
			case "rreqs":
				rchClosed = true
			}

			if lchEOF && lchClosed && !rchClosed {
				rch.Close()
			}

			if rchEOF && rchClosed && !lchClosed {
				lch.Close()
			}

			if lchEOF && rchEOF && lchClosed && rchClosed {
				return nil
			}

			return nil
		}
	}
}
