package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/gliderlabs/ssh"
	"github.com/rs/zerolog/log"
	"github.com/werbot/werbot/api/proto/server"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/pkg/netutil"

	pb "github.com/werbot/werbot/api/proto/server"
)

func privateKey() func(*ssh.Server) error {
	return func(srv *ssh.Server) error {
		privateBytes, err := os.ReadFile(internal.GetString("SSHSERVER_PIPER_KEY_FILE", "/server.key"))
		if err != nil {
			app.log.Error(err).Msg("Don't open piper key file")
		}

		private, err := gossh.ParsePrivateKey(privateBytes)
		if err != nil {
			app.log.Error(err).Msg("Don't parse piper key file")
		}
		srv.AddHostKey(private)
		return nil
	}
}

func dynamicHostKey(host *server.Server_Response) gossh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		if len(host.HostKey) == 0 {
			log.Info().Str("hostAddress", netutil.IP(hostname)).Msg("Discovering host fingerprint")

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			rClient := pb.NewServerHandlersClient(app.grpc.Client)

			_, err := rClient.UpdateServerHostKey(ctx, &server.UpdateServerHostKey_Request{
				ServerId: host.ServerId,
				Hostkey:  key.Marshal(),
			})
			if err != nil {
				app.log.Error(err).Msg("gRPC UpdateServerHostKey")
			}
			return nil
		}

		if !bytes.Equal(host.HostKey, key.Marshal()) {
			return errors.New("SSH host key mismatch")
		}
		return nil
	}
}
