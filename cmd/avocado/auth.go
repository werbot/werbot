package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/api/proto/account"
	"github.com/werbot/werbot/api/proto/firewall"
	"github.com/werbot/werbot/api/proto/server"
	"github.com/werbot/werbot/pkg/netutil"
	"github.com/werbot/werbot/pkg/strutil"

	pb_account "github.com/werbot/werbot/api/proto/account"
	pb_firewall "github.com/werbot/werbot/api/proto/firewall"
)

func passwordAuthHandler() ssh.PasswordHandler {
	return func(ctx ssh.Context, pass string) bool {
		actx := &authContext{
			userName:   ctx.User(),
			userAddr:   ctx.RemoteAddr().String(),
			authMethod: "password",
		}
		actx.authSuccess = actx.userType() == server.UserType_HEALTHCHECK
		ctx.SetValue(authContextKey, actx)
		return actx.authSuccess
	}
}

func publicKeyAuthHandler() ssh.PublicKeyHandler {
	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		actx := &authContext{
			userName:        fixUsername(ctx.User()),
			userAddr:        netutil.IP(ctx.RemoteAddr().String()),
			userFingerPrint: gossh.FingerprintLegacyMD5(key),
			uuid:            uuid.New().String(),
			authMethod:      "pubkey",
			authSuccess:     true,
		}
		ctx.SetValue(authContextKey, actx)

		_ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		rClientF := pb_firewall.NewFirewallHandlersClient(app.grpc.Client)
		rClientA := pb_account.NewAccountHandlersClient(app.grpc.Client)

		// IP check for global Faerwole settings
		status, err := rClientF.CheckIPAccess(_ctx, &firewall.CheckIPAccess_Request{
			ClientIp: actx.userAddr,
		})
		if err != nil {
			actx.message = "Access denied"
			actx.authSuccess = false
			return true
		}

		// for some reason it paint if you combine these two designs if
		if !status.Access {
			actx.message = "Access denied"
			actx.authSuccess = false
			return true
		}

		actx.userCountry = status.Country

		// Checking the syntax of writing login
		if !checkUsername(actx.userName) {
			actx.message = "Violated username syntax"
			actx.authSuccess = false
			return true
		}

		switch actx.userType() {
		case server.UserType_INVITE:
			inputToken := strings.Split(actx.userName, "_")[1]
			if len(inputToken) > 0 {
				fmt.Print(inputToken)
			}
			return true

		case server.UserType_HEALTHCHECK:
			return true
		}

		userID, err := rClientA.AccountIDByName(_ctx, &account.AccountIDByName_Request{
			Username:    actx.userName,
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

func checkUsername(user string) bool {
	unixUserRegexp := regexp.MustCompile("^[a-z_][a-zA-Z0-9_]{0,31}$")
	return unixUserRegexp.MatchString(user)
}

func fixUsername(user string) string {
	username := strutil.SplitTrimmed(user, "_")
	return strings.Join(username, "_")
}
