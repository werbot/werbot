package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	firewallpb "github.com/werbot/werbot/internal/core/firewall/proto/firewall"
	profilepb "github.com/werbot/werbot/internal/core/profile/proto/profile"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/utils/alias"
	"github.com/werbot/werbot/pkg/utils/netutil"
)

func passwordAuthHandler() ssh.PasswordHandler {
	return func(ctx ssh.Context, pass string) bool {
		actx := &authContext{
			alias:      ctx.User(),
			userAddr:   ctx.RemoteAddr().String(),
			authMethod: "password",
		}
		actx.authSuccess = actx.userType() == schemepb.Type_healthcheck
		ctx.SetValue(authContextKey, actx)
		return actx.authSuccess
	}
}

func publicKeyAuthHandler() ssh.PublicKeyHandler {
	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		actx := &authContext{
			alias:           alias.FixAlias(ctx.User()),
			userAddr:        netutil.IP(ctx.RemoteAddr().String()),
			userFingerPrint: gossh.FingerprintLegacyMD5(key),
			authMethod:      "pubkey",
			authSuccess:     true,
		}
		ctx.SetValue(authContextKey, actx)

		_ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		rClientF := firewallpb.NewFirewallHandlersClient(app.grpc)
		rClientA := profilepb.NewProfileHandlersClient(app.grpc)

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
		if !alias.CheckAlias(actx.alias) {
			actx.message = "Violated alias syntax"
			actx.authSuccess = false
			return true
		}

		switch actx.userType() {
		case schemepb.Type_invite:
			inputToken := strings.Split(actx.alias, "_")[1]
			if len(inputToken) > 0 {
				fmt.Print(inputToken)
			}
			return true

		case schemepb.Type_healthcheck:
			return true
		}

		profileID, err := rClientA.ProfileIDByLogin(_ctx, &profilepb.ProfileIDByLogin_Request{
			Login:       actx.alias,
			Fingerprint: actx.userFingerPrint,
			ClientIp:    actx.userAddr,
		})
		if err != nil {
			return true
		}

		actx.userID = profileID.GetProfileId()
		if actx.userID != "" {
			return true
		}

		return true
	}
}
