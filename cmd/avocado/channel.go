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
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/grpc/proto/account"
	"github.com/werbot/werbot/internal/grpc/proto/audit"
	"github.com/werbot/werbot/internal/grpc/proto/firewall"
	"github.com/werbot/werbot/internal/grpc/proto/server"
	"github.com/werbot/werbot/internal/service/ssh/auditor"
	"github.com/werbot/werbot/internal/service/ssh/pty"
	"github.com/werbot/werbot/internal/utils/convert"
	"github.com/werbot/werbot/internal/utils/parse"

	pb_account "github.com/werbot/werbot/internal/grpc/proto/account"
	pb_audit "github.com/werbot/werbot/internal/grpc/proto/audit"
	pb_firewall "github.com/werbot/werbot/internal/grpc/proto/firewall"
	pb_server "github.com/werbot/werbot/internal/grpc/proto/server"
)

type sessionConfig struct {
	ClientConfig *gossh.ClientConfig
	Channel      channelTunnel

	Addr      string
	AccountID string
	ClientIP  string
	UUID      string
}

type channelTunnel struct {
	lch   gossh.Channel
	lreqs <-chan *gossh.Request
	err   error
}

// TODO: объединить host и actx с необходимыми параметрами в один
// https://git.piplos.by/werbot/werbot-server/blob/3c833b2e6fd5a5d2914a4d9aaa640040dd605371/server.go
func connectToHost(host *server.GetServer_Response, actx *authContext, ctx ssh.Context, newChan gossh.NewChannel, srv *ssh.Server, conn *gossh.ServerConn, ch channelTunnel) {
	_ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClientF := pb_firewall.NewFirewallHandlersClient(app.grpc.Client)
	rClientS := pb_server.NewServerHandlersClient(app.grpc.Client)
	rClientA := pb_account.NewAccountHandlersClient(app.grpc.Client)

	status, err := rClientF.CheckServerAccess(_ctx, &firewall.CheckServerAccess_Request{
		AccountId: host.ServerId,
		UserId:    actx.userID,
		Country:   actx.userCountry,
		ClientIp:  actx.userAddr,
	})
	if err != nil {
		actx.message = "Problem with server access"
		sendMessageInChannel(ch.lch, actx.message+"\n")
		ch.lch.Close()
		return
	}
	if !status.Access {
		actx.message = "Permission denied"
		sendMessageInChannel(ch.lch, actx.message+"\n")
		ch.lch.Close()
		return
	}

	switch host.Scheme {
	case "ssh":
		sessionConfigs := make([]sessionConfig, 0)
		clientConfig, err := bastionClientConfig(ctx, host)
		if err != nil {
			log.Error().Err(err).Msg("Bastion ClientConfig error")
			actx.message = fmt.Sprintf("%v", err)
			sendMessageInChannel(ch.lch, actx.message+"\n")
			ch.lch.Close()
			return
		}

		// app.nats.AccountStatus(host.AccountId, "online")
		_, err = rClientA.SetAccountStatus(_ctx, &account.SetAccountStatus_Request{
			AccountId: host.AccountId,
			Status:    2, // online
		})
		if err != nil {
			log.Error().Err(err).Msg("gRPC SetAccountStatus")
		}

		_, err = rClientS.CreateServerSession(_ctx, &server.CreateServerSession_Request{
			AccountId: host.AccountId,
			Status:    server.SessionStatus_OPENED,
			Message:   "",
			Uuid:      actx.uuid,
		})
		if err != nil {
			log.Error().Err(err).Msg("gRPC ServerSessionAdd")
		}

		_, err = rClientS.UpdateServerOnlineStatus(_ctx, &server.UpdateServerOnlineStatus_Request{
			ServerId: host.ServerId,
			Status:   true,
		})
		if err != nil {
			log.Error().Err(err).Msg("gRPC UpdateServerOnlineStatus")
		}

		log.Info().Str("userName", actx.userName).Str("userAddress", actx.userAddr).Str("hostID", actx.hostID).Str("userID", actx.userID).Str("UUID", actx.uuid).Msg("Open virtual connection")

		sessionConfigs = append([]sessionConfig{{
			Addr:         actx.hostAddr,
			ClientConfig: clientConfig,
			AccountID:    host.AccountId,
			ClientIP:     actx.userAddr,
			UUID:         actx.uuid,
			Channel:      ch,
		}}, sessionConfigs...)

		go func() {
			err = multiChannelHandler(srv, conn, newChan, ctx, sessionConfigs)
			if err != nil {
				log.Error().Err(err).Msg("Multi ChannelHandler error")
				_, err := rClientS.UpdateServerOnlineStatus(_ctx, &server.UpdateServerOnlineStatus_Request{
					ServerId: host.ServerId,
					Status:   false,
				})
				if err != nil {
					log.Error().Err(err).Msg("gRPC UpdateServerOnlineStatus")
				}

				conn.Close()
			}

			_, err = rClientS.CreateServerSession(_ctx, &server.CreateServerSession_Request{
				AccountId: host.AccountId,
				Status:    server.SessionStatus_CLOSED,
				Message:   actx.message,
				Uuid:      actx.uuid,
			})
			if err != nil {
				log.Error().Err(err).Msg("gRPC ServerSessionAdd")
			}

			// app.nats.AccountStatus(host.AccountId, "offline")
			_, err := rClientA.SetAccountStatus(_ctx, &account.SetAccountStatus_Request{
				AccountId: host.AccountId,
				Status:    1, // offline
			})
			if err != nil {
				log.Error().Err(err).Msg("gRPC SetAccountStatus")
			}

			actx.message = "Host unavailable"
			log.Info().Str("userName", actx.userName).Str("userAddress", actx.userAddr).Str("hostID", actx.hostID).Str("userID", actx.userID).Str("UUID", actx.uuid).Msg("Closed virtual connection")
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
			log.Error().Err(err).Msg("Failed to reject channel")
		}
		return
	}

	actx := ctx.Value(authContextKey).(*authContext)

	ch, req, err := newChan.Accept()
	if err != nil {
		log.Error().Err(err).Msg("Could not accept channel")
		return
	}

	_ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb_server.NewServerHandlersClient(app.grpc.Client)

	switch actx.userType() {
	// case config.UserTypeHealthcheck:
	case server.UserType_HEALTHCHECK:
		sendMessageInChannel(ch, "OK\n")
		_ = ch.Close()
		return

	// TODO: добавить одноразовые инвйты
	// case config.UserTypeInvite:
	case server.UserType_INVITE:
		log.Info().Str("invite", actx.message).Str("userAddress", actx.userAddr).Msg("Invite is invalid")
		sendMessageInChannel(ch, fmt.Sprintf("Invite %s is invalid.\n", actx.message))
		_ = ch.Close()
		return

	// case config.UserTypeShell:
	case server.UserType_SHELL:
		if actx.userID == "" {
			log.Info().Str("userName", actx.userName).Str("userAddress", actx.userAddr).Msg("Permission denied")
			actx.message = "Firewall denied access"
			sendMessageInChannel(ch, actx.message+"\n")
			_ = ch.Close()
			return
		}

		sendMessageInChannel(ch, " _    _  ____  ____  ____  _____  ____ \n( \\/\\/ )( ___)(  _ \\(  _ \\(  _  )(_  _)\n \033[0;31m)    (  )__)  )   / ) _ < )(_)(   )(\033[0m  \n(__/\\__)(____)(_)\\_)(____/(_____) (__) \n"+internal.Version()+", "+internal.Commit()+", "+internal.BuildDate()+"\n\n")

		serverList, _ := rClient.ListServers(_ctx, &server.ListServers_Request{
			Query: "user_name=" + actx.userName,
		})
		actx.serverList = serverList.GetServers()

		if len(actx.serverList) > 0 {
			log.Info().Str("userName", actx.userName).Str("userAddress", actx.userAddr).Str("userID", actx.userID).Str("UUID", actx.uuid).Msg("Open shellconsole connection")

			nameArray := parse.UsernameParseInfo(actx.userName)
			status := map[bool]string{
				false: "\x1B[01;31m•\x1B[0m",
				true:  "\x1B[01;32m•\x1B[0m",
			}

			message := "Hello " + nameArray[0] + ", you access to next servers:\n"
			bufMsg := &strings.Builder{}
			table := tablewriter.NewWriter(bufMsg)
			table.SetHeader([]string{"⚡", "Name", "Login for direct access", "🚚"})
			table.SetAutoWrapText(false)

			for i := 0; i < len(actx.serverList); i++ {
				server := actx.serverList[int32(i)]
				table.Append([]string{fmt.Sprintf("%v %v", status[server.Online], (i + 1)), server.Title, fmt.Sprintf("%v_%v_%v", nameArray[0], server.ProjectLogin, server.Token), server.Scheme})
			}
			table.Render()
			message += bufMsg.String()
			sendMessageInChannel(ch, message)

			term := term.NewTerminal(ch, "Select server or push enter to exit > ")
			selectedServer, _ := term.ReadLine()
			selectServer := convert.StringToInt32(selectedServer)

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

	case server.UserType_BASTION:
		if actx.userID == "" {
			log.Info().Str("userName", actx.userName).Str("userAddress", actx.userAddr).Msg("Permission denied")
			actx.message = "Permission denied"
			sendMessageInChannel(ch, actx.message+"\n")
			_ = ch.Close()
			return
		}

		getHosts, err := rClient.ListServers(_ctx, &server.ListServers_Request{
			Query: "user_name=" + actx.userName,
		})
		if err != nil {
			log.Error().Err(err).Str("userName", actx.userName).Str("userAddress", actx.userAddr).Msg("Host not found (channel)")
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
			log.Error().Err(err).Msg("failed to reject chan")
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

	newAudit := &audit.CreateAudit_Request{
		AccountId: sessConfig.AccountID,
		ClientIp:  sessConfig.ClientIP,
		Session:   sessConfig.UUID,
	}

	wrappedlch := auditor.NewLogchannel(newAudit, lch, app.grpc, int32(internal.GetInt("SSHSERVER_RECORD_COUNT", 50)))
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
	rClient := pb_audit.NewAuditHandlersClient(app.grpc.Client)

	go func(quit chan string) {
		for req := range lreqs {
			b, err := rch.SendRequest(req.Type, req.WantReply, req.Payload)
			if req.Type == "exec" {
				wrappedlch := auditor.NewLogchannel(newAudit, lch, app.grpc, int32(internal.GetInt("SSHSERVER_RECORD_COUNT", 50)))
				command := append(req.Payload, []byte("\n")...)
				if _, err := wrappedlch.Write(command); err != nil {
					log.Error().Err(err).Msg("failed to write log")
				}
			}

			if req.Type == "pty-req" {
				ptyReq, _ := pty.ParsePtyRequest(req.Payload)

				_, err = rClient.UpdateAudit(_ctx, &audit.UpdateAudit_Request{
					AuditId: auditID,
					Params: &audit.UpdateAudit_Request_Params{
						Width:   int32(ptyReq.Width),
						Height:  int32(ptyReq.Height),
						EnvTerm: ptyReq.EnvTerm,
					},
				})
				if err != nil {
					log.Error().Err(err).Msg("gRPC AuditUpdate")
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
