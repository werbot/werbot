package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/core/scheme/proto/access"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/utils/strutil"
)

var authContextKey = sshportalContextKey("auth")

type sshportalContextKey string

type authContext struct {
	message         string
	authMethod      string
	authSuccess     bool
	sessionID       string
	userID          string
	alias           string
	userFingerprint string
	schemeID        string
	clientIP        string
	serverList      []*schemepb.Scheme_Response
}

func (c authContext) userType() schemepb.Type {
	if c.alias == "healthcheck" {
		return schemepb.Type_healthcheck
	}

	if strings.HasPrefix(c.alias, "invite_") {
		return schemepb.Type_invite
	}

	loginArray := strutil.SplitNTrimmed(c.alias, "_", 3)
	if len(loginArray) == 3 && loginArray[2] != "" {
		return schemepb.Type_bastion
	}
	return schemepb.Type_shell
}

// schemeAccess fetches the scheme connection settings with secrets decrypted.
func schemeAccess(ctx context.Context, schemeID, clientIP string) (*access.AccessScheme_Server_SSH, error) {
	rClient := schemepb.NewSchemeHandlersClient(app.grpc)
	resp, err := rClient.SystemSchemeAccess(ctx, &schemepb.SystemSchemeAccess_Request{
		SchemeId: schemeID,
		ClientIp: clientIP,
		NoGhost:  true,
	})
	if err != nil {
		return nil, err
	}

	serverSSH := resp.GetAccess().GetServerSsh()
	if serverSSH == nil {
		return nil, errors.New("scheme is not an SSH server")
	}

	aesKey := internal.GetString("SECURITY_AES_KEY", "3D6A619811A17396E45D514695DCDA3A")
	switch auth := serverSSH.GetAccess().(type) {
	case *access.AccessScheme_Server_SSH_Password:
		auth.Password.Password = crypto.TextDecrypt(auth.Password.GetPassword(), aesKey)
		if isMasked(auth.Password.GetPassword()) {
			return nil, errors.New("scheme password is masked by the server")
		}
	case *access.AccessScheme_Server_SSH_Key:
		auth.Key.GetKey().Private = crypto.TextDecrypt(auth.Key.GetKey().GetPrivate(), aesKey)
		auth.Key.GetKey().Passphrase = crypto.TextDecrypt(auth.Key.GetKey().GetPassphrase(), aesKey)
		if isMasked(auth.Key.GetKey().GetPrivate()) {
			return nil, errors.New("scheme private key is masked by the server")
		}
	}

	return serverSSH, nil
}

// isMasked reports whether the server returned the ghoster secret mask instead of a real value.
func isMasked(secret string) bool {
	return secret == "***"
}

// session is the resolved target address plus its SSH client config.
type session struct {
	Address      string
	ClientConfig *gossh.ClientConfig
}

// newSession resolves scheme access, decrypts secrets and builds the SSH client config.
func newSession(scheme *schemepb.Scheme_Response, actx *authContext, ctx context.Context) (*session, error) {
	serverSSH, err := schemeAccess(ctx, scheme.GetSchemeId(), actx.clientIP)
	if err != nil {
		return nil, err
	}

	authMethods := []gossh.AuthMethod{}
	login := ""

	switch auth := serverSSH.GetAccess().(type) {
	case *access.AccessScheme_Server_SSH_Password:
		login = auth.Password.GetLogin()
		if auth.Password.GetPassword() != "" {
			authMethods = append(authMethods, gossh.Password(auth.Password.GetPassword()))
		}

	case *access.AccessScheme_Server_SSH_Key:
		login = auth.Key.GetLogin()
		key := auth.Key.GetKey()
		if key.GetPrivate() == "" {
			break
		}

		var signer gossh.Signer
		if key.GetPassphrase() != "" {
			signer, err = gossh.ParsePrivateKeyWithPassphrase([]byte(key.GetPrivate()), []byte(key.GetPassphrase()))
		} else {
			signer, err = gossh.ParsePrivateKey([]byte(key.GetPrivate()))
		}
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		authMethods = append(authMethods, gossh.PublicKeys(signer))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no valid authentication method for scheme %q", scheme.GetTitle())
	}

	return &session{
		Address: fmt.Sprintf("%s:%d", serverSSH.GetAddress(), serverSSH.GetPort()),
		ClientConfig: &gossh.ClientConfig{
			User:            login,
			HostKeyCallback: dynamicHostKey(scheme),
			Auth:            authMethods,
			Timeout:         time.Duration(internal.GetInt("SSHSERVER_IDLE_TIMEOUT", 300)) * time.Second,
		},
	}, nil
}
