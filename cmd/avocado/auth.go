package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	firewallpb "github.com/werbot/werbot/internal/grpc/firewall/proto"
	serverpb "github.com/werbot/werbot/internal/grpc/server/proto"
	"github.com/werbot/werbot/pkg/netutil"
	"github.com/werbot/werbot/pkg/strutil"
)

func passwordAuthHandler() ssh.PasswordHandler {
	return func(ctx ssh.Context, pass string) bool {
		actx := &authContext{
			login:      ctx.User(),
			userAddr:   ctx.RemoteAddr().String(),
			authMethod: "password",
		}
		actx.authSuccess = actx.userType() == serverpb.Type_healthcheck
		ctx.SetValue(authContextKey, actx)
		return actx.authSuccess
	}
}

func publicKeyAuthHandler() ssh.PublicKeyHandler {
	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		actx := &authContext{
			login:           fixLogin(ctx.User()),
			userAddr:        netutil.IP(ctx.RemoteAddr().String()),
			userFingerPrint: gossh.FingerprintLegacyMD5(key),
			authMethod:      "pubkey",
			authSuccess:     true,
		}
		ctx.SetValue(authContextKey, actx)

		_ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		rClientF := firewallpb.NewFirewallHandlersClient(app.grpc.Client)
		rClientA := accountpb.NewAccountHandlersClient(app.grpc.Client)

		// IP check for global Firewall settings
		_, err := rClientF.IPAccess(_ctx, &firewallpb.IPAccess_Request{
			ClientIp: actx.userAddr,
		})
		if err != nil {
			actx.message = "Access denied"
			actx.authSuccess = false
			return true
		}

		// Checking the syntax of writing login
		if !checkLogin(actx.login) {
			actx.message = "Violated login syntax"
			actx.authSuccess = false
			return true
		}

		switch actx.userType() {
		case serverpb.Type_invite:
			inputToken := strings.Split(actx.login, "_")[1]
			if len(inputToken) > 0 {
				fmt.Print(inputToken)
			}
			return true

		case serverpb.Type_healthcheck:
			return true
		}

		userID, err := rClientA.AccountIDByLogin(_ctx, &accountpb.AccountIDByLogin_Request{
			Login:       actx.login,
			Fingerprint: actx.userFingerPrint,
			ClientIp:    actx.userAddr,
		})
		if err != nil {
			return true
		}

		actx.userID = userID.GetUserId()
		if actx.userID != "" {
			return true
		}

		return true
	}
}

func checkLogin(login string) bool {
	unixUserRegexp := regexp.MustCompile("^[a-z_][a-zA-Z0-9_]{0,31}$")
	return unixUserRegexp.MatchString(login)
}

func fixLogin(login string) string {
	_login := strutil.SplitTrimmed(login, "_")
	return strings.Join(_login, "_")
}
