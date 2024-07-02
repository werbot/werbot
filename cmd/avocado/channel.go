package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/olekukonko/tablewriter"
	"github.com/rs/zerolog/log"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"github.com/werbot/werbot/internal"
	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	auditpb "github.com/werbot/werbot/internal/grpc/audit/proto"
	firewallpb "github.com/werbot/werbot/internal/grpc/firewall/proto"
	serverpb "github.com/werbot/werbot/internal/grpc/server/proto"
	"github.com/werbot/werbot/internal/service/ssh/auditor"
	"github.com/werbot/werbot/internal/service/ssh/pty"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/version"
	"github.com/werbot/werbot/pkg/strutil"
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

// TODO: combine Host and Actx with the required parameters in one
// https://git.piplos.by/werbot/werbot-server/blob/3c833b2e6fd5a5d2914a4d9aaa640040dd605371/server.go
func connectToHost(host *serverpb.Server_Response, actx *authContext, ctx ssh.Context, newChan gossh.NewChannel, srv *ssh.Server, conn *gossh.ServerConn, ch channelTunnel) {
	_ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClientF := firewallpb.NewFirewallHandlersClient(app.grpc)
	rClientS := serverpb.NewServerHandlersClient(app.grpc)
	rClientA := accountpb.NewAccountHandlersClient(app.grpc)

	_, err := rClientF.ServerAccess(_ctx, &firewallpb.ServerAccess_Request{
		ServerId: host.ServerId,
		UserId:   actx.userID,
		MemberIp: actx.userAddr,
	})
	if err != nil {
		actx.message = "Permission denied"
		sendMessageInChannel(ch.lch, actx.message+"\n")
		ch.lch.Close()
		return
	}

	switch host.GetScheme() {
	case serverpb.ServerScheme_ssh: // ssh
		sessionConfigs := make([]sessionConfig, 0)
		clientConfig, err := bastionClientConfig(ctx, host)
		if err != nil {
			app.log.Error(err).Msg("Bastion ClientConfig error")
			actx.message = fmt.Sprintf("%v", err)
			sendMessageInChannel(ch.lch, actx.message+"\n")
			ch.lch.Close()
			return
		}

		app.broker.AccountStatus(host.AccountId, "online")

		_, err = rClientA.UpdateStatus(_ctx, &accountpb.UpdateStatus_Request{
			AccountId: host.AccountId,
			Status:    2, // online
		})
		if err != nil {
			app.log.Error(err).Msg("gRPC UpdateAccountStatus")
		}

		data, err := rClientS.AddSession(_ctx, &serverpb.AddSession_Request{
			AccountId: host.AccountId,
			Status:    serverpb.SessionStatus_opened,
			Message:   "",
		})
		if err != nil {
			app.log.Error(err).Msg("gRPC ServerSessionAdd")
		}
		actx.sessionID = data.GetSessionId()

		_, err = rClientS.UpdateServer(_ctx, &serverpb.UpdateServer_Request{
			ServerId: host.ServerId,
			Setting: &serverpb.UpdateServer_Request_Active{
				Active: true,
			},
		})
		if err != nil {
			app.log.Error(err).Msg("gRPC UpdateServerOnlineStatus")
		}

		log.Info().Str("login", actx.login).Str("userAddress", actx.userAddr).Str("hostID", actx.hostID).Str("userID", actx.userID).Str("UUID", actx.sessionID).Msg("Open virtual connection")

		sessionConfigs = append([]sessionConfig{{
			Addr:         actx.hostAddr,
			ClientConfig: clientConfig,
			AccountID:    host.AccountId,
			ClientIP:     actx.userAddr,
			SessionID:    actx.sessionID,
			Channel:      ch,
		}}, sessionConfigs...)

		go func() {
			err = multiChannelHandler(srv, conn, newChan, ctx, sessionConfigs)
			if err != nil {
				app.log.Error(err).Msg("Multi ChannelHandler error")
				_, err := rClientS.UpdateServer(_ctx, &serverpb.UpdateServer_Request{
					ServerId: host.ServerId,
					Setting: &serverpb.UpdateServer_Request_Active{
						Active: false,
					},
				})
				if err != nil {
					app.log.Error(err).Msg("gRPC UpdateServerOnlineStatus")
				}

				conn.Close()
			}

			data, err = rClientS.AddSession(_ctx, &serverpb.AddSession_Request{
				AccountId: host.AccountId,
				Status:    serverpb.SessionStatus_closed,
				Message:   actx.message,
			})
			if err != nil {
				app.log.Error(err).Msg("gRPC ServerSessionAdd")
			}
			actx.sessionID = data.GetSessionId()

			app.broker.AccountStatus(host.AccountId, "offline")

			_, err := rClientA.UpdateStatus(_ctx, &accountpb.UpdateStatus_Request{
				AccountId: host.AccountId,
				Status:    1, // offline
			})
			if err != nil {
				app.log.Error(err).Msg("gRPC UpdateAccountStatus")
			}

			actx.message = "Host unavailable"
			app.log.Info().Str("login", actx.login).Str("userAddress", actx.userAddr).Str("hostID", actx.hostID).Str("userID", actx.userID).Str("UUID", actx.sessionID).Msg("Closed virtual connection")
		}()

	// case "telnet":
	//	tmpSrv := ssh.Server{
	//		Handler: telnetHandler(host),
	//	}
	//	defaultChannelHandler(&tmpSrv, conn, newChan, ctx)
	default:
		actx.message = "unknown bastion scheme"
		sendMessageInChannel(ch.lch, actx.message+"\n")
	}
}

func channelHandler(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
	switch newChan.ChannelType() {
	case "session":
	case "direct-tcpip":
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

	rClient := serverpb.NewServerHandlersClient(app.grpc)

	switch actx.userType() {
	// case config.UserTypeHealthcheck:
	case serverpb.Type_healthcheck:
		sendMessageInChannel(ch, "OK\n")
		_ = ch.Close()
		return

	// TODO: Add disposable invites
	// case config.UserTypeInvite:
	case serverpb.Type_invite:

		app.log.Info().Str("invite", actx.message).Str("userAddress", actx.userAddr).Msg(trace.MsgInviteIsInvalid)
		sendMessageInChannel(ch, fmt.Sprintf("Invite %s is invalid.\n", actx.message))
		_ = ch.Close()
		return

	// case config.UserTypeShell:
	case serverpb.Type_shell:
		if actx.userID == "" {
			app.log.Info().Str("login", actx.login).Str("userAddress", actx.userAddr).Msg(trace.MsgAccessIsDeniedUser)
			actx.message = "Firewall denied access"
			sendMessageInChannel(ch, actx.message+"\n")
			_ = ch.Close()
			return
		}

		sendMessageInChannel(ch, " _    _  ____  ____  ____  _____  ____ \n( \\/\\/ )( ___)(  _ \\(  _ \\(  _  )(_  _)\n \033[0;31m)    (  )__)  )   / ) _ < )(_)(   )(\033[0m  \n(__/\\__)(____)(_)\\_)(____/(_____) (__) \n"+version.Version()+", "+version.Commit()+", "+version.BuildDate()+"\n\n")

		serverList, _ := rClient.ListServers(_ctx, &serverpb.ListServers_Request{
			Login: actx.login,
		})
		actx.serverList = serverList.GetServers()

		if len(actx.serverList) > 0 {
			app.log.Info().Str("login", actx.login).Str("userAddress", actx.userAddr).Str("userID", actx.userID).Str("UUID", actx.sessionID).Msg("Open shellconsole connection")

			loginArray := strutil.SplitNTrimmed(actx.login, "_", 3)
			status := map[bool]string{
				false: "\x1B[01;31m•\x1B[0m",
				true:  "\x1B[01;32m•\x1B[0m",
			}

			message := "Hello " + loginArray[0] + ", you access to next servers:\n"
			bufMsg := &strings.Builder{}
			table := tablewriter.NewWriter(bufMsg)
			table.SetHeader([]string{"⚡", "Name", "Login for direct access", "🚚"})
			table.SetAutoWrapText(false)

			for i := 0; i < len(actx.serverList); i++ {
				server := actx.serverList[int32(i)]
				table.Append([]string{fmt.Sprintf("%v %v", status[server.Online], (i + 1)), server.Title, fmt.Sprintf("%v_%v_%v", loginArray[0], server.ProjectLogin, server.Token), server.GetScheme().String()})
			}
			table.Render()
			message += bufMsg.String()
			sendMessageInChannel(ch, message)

			term := term.NewTerminal(ch, "Select server or push enter to exit > ")
			selectedServer, _ := term.ReadLine()
			selectServer := strutil.ToInt32(selectedServer)

			switch {
			case selectServer <= int32(len(actx.serverList)) && selectServer > 0:
				selectServer--
				actx.hostAddr = fmt.Sprintf("%v:%d", actx.serverList[selectServer].Address, actx.serverList[selectServer].Port)
				actx.hostID = actx.serverList[selectServer].ServerId
				sendMessageInChannel(ch, "Connect to server-name: "+actx.serverList[selectServer].Title+"\n")
				connectToHost(actx.serverList[selectServer], actx, ctx, newChan, srv, conn, channelTunnel{ch, req, err})

			case selectServer > int32(len(actx.serverList)):
				sendMessageInChannel(ch, "\nServer was not found\nBye-Bye! :(\n\n")
				_ = ch.Close()

			default:
				sendMessageInChannel(ch, "\nYou have not selected server\nBye-Bye! :(\n\n")
				_ = ch.Close()
			}
		} else {
			sendMessageInChannel(ch, "\nYou don't have active servers\nBye-Bye! :(\n\n")
			_ = ch.Close()
		}
		return

	case serverpb.Type_bastion:
		if actx.userID == "" {
			app.log.Info().Str("login", actx.login).Str("userAddress", actx.userAddr).Msg("Permission denied")
			actx.message = "Permission denied"
			sendMessageInChannel(ch, actx.message+"\n")
			_ = ch.Close()
			return
		}

		getHosts, err := rClient.ListServers(_ctx, &serverpb.ListServers_Request{
			Login: actx.login,
		})
		if err != nil {
			app.log.Error(err).Str("login", actx.login).Str("userAddress", actx.userAddr).Msg("Host not found (channel)")
			actx.message = "Host not found"
			sendMessageInChannel(ch, actx.message+"\n")
			_ = ch.Close()
			return
		}
		host := getHosts.Servers[0]

		actx.hostAddr = fmt.Sprintf("%v:%d", host.Address, host.Port)
		actx.hostID = host.ServerId
		connectToHost(host, actx, ctx, newChan, srv, conn, channelTunnel{ch, req, err})
		return

	default:
		app.defaultChannelHandler(srv, conn, newChan, ctx)
	}

	_ = ch.Close()
}

func multiChannelHandler(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context, configs []sessionConfig) error {
	var lastClient *gossh.Client

	switch newChan.ChannelType() {
	case "session":
		lch, lreqs, err := configs[0].Channel.lch, configs[0].Channel.lreqs, configs[0].Channel.err
		if err != nil {
			return errors.New("Duplicate response received for channel")
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

		rch, rreqs, err := lastClient.OpenChannel("session", []byte{})
		if err != nil {
			return err
		}
		return pipe(lreqs, rreqs, lch, rch, newChan, configs[0])

	case "direct-tcpip":
		lch, lreqs, err := configs[0].Channel.lch, configs[0].Channel.lreqs, configs[0].Channel.err
		if err != nil {
			return errors.New("Duplicate response received for channel")
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

		d := auditor.ForwardData{}
		if err := gossh.Unmarshal(newChan.ExtraData(), &d); err != nil {
			return err
		}
		rch, rreqs, err := lastClient.OpenChannel("direct-tcpip", newChan.ExtraData())
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

	newAudit := &auditpb.AddAudit_Request{
		AccountId: sessConfig.AccountID,
		ClientIp:  sessConfig.ClientIP,
		Session:   sessConfig.SessionID,
	}

	recordCount := internal.GetInt32("SSHSERVER_RECORD_COUNT", 50)
	wrappedlch := auditor.NewLogchannel(newAudit, lch, app.grpc, recordCount)
	auditID := wrappedlch.AuditID
	defer wrappedlch.Close()

	channeltype := newChan.ChannelType()
	if channeltype == "session" {
		go func(quit chan string) {
			_, _ = io.Copy(wrappedlch, rch)
			quit <- "rch"
		}(quit)

		go func(quit chan string) {
			_, _ = io.Copy(rch, lch)
			quit <- "lch"
		}(quit)
	}

	if channeltype == "direct-tcpip" {
		d := auditor.ForwardData{}
		if err := gossh.Unmarshal(newChan.ExtraData(), &d); err != nil {
			return err
		}
		wrappedlch := auditor.NewLogtunnel(lch, lch, d.SourceHost)
		wrappedrch := auditor.NewLogtunnel(rch, lch, d.DestinationHost)
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

	rClient := auditpb.NewAuditHandlersClient(app.grpc)

	go func(quit chan string) {
		for req := range lreqs {
			b, err := rch.SendRequest(req.Type, req.WantReply, req.Payload)
			if req.Type == "exec" {
				recordCount := internal.GetInt32("SSHSERVER_RECORD_COUNT", 50)
				wrappedlch := auditor.NewLogchannel(newAudit, lch, app.grpc, recordCount)
				command := append(req.Payload, []byte("\n")...)
				if _, err := wrappedlch.Write(command); err != nil {
					app.log.Error(err).Msg("failed to write log")
				}
			}

			if req.Type == "pty-req" {
				ptyReq, _ := pty.ParsePtyRequest(req.Payload)

				_, err = rClient.UpdateAudit(_ctx, &auditpb.UpdateAudit_Request{
					AuditId: auditID,
					Width:   int32(ptyReq.Width),
					Height:  int32(ptyReq.Height),
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

			// TODO: отладить закрытие, возникает ошибка при обрыве сессии из-за которой не срабатывает нормально конструкция ниже
			// жёстко закрываем все
			// rch.Close()
			// lch.Close()

			if lchEOF && lchClosed && !rchClosed {
				rch.Close()
			}

			if rchEOF && rchClosed && !lchClosed {
				lch.Close()
			}

			if lchEOF && rchEOF && lchClosed && rchClosed {
				return nil
			}

			// TODO: принудительно без ошибок :)
			return nil
		}
	}
}
