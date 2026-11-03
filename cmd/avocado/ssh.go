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

	aesKey := internal.GetString("SECURITY_AES_KEY", "")
	if !crypto.CheckAESKey(aesKey) {
		return nil, errors.New("SECURITY_AES_KEY must be a hex encoded 16, 24 or 32 byte AES key")
	}

	switch auth := serverSSH.GetAccess().(type) {
	case *access.AccessScheme_Server_SSH_Password:
		if isMasked(auth.Password.GetPassword()) {
			return nil, errors.New("scheme password is masked by the server")
		}
		if auth.Password.GetPassword() != "" {
			password, err := crypto.TextDecrypt(auth.Password.GetPassword(), aesKey)
			if err != nil {
				return nil, fmt.Errorf("decrypt scheme password: %w", err)
			}
			auth.Password.Password = password
		}
	case *access.AccessScheme_Server_SSH_Key:
		if isMasked(auth.Key.GetKey().GetPrivate()) {
			return nil, errors.New("scheme private key is masked by the server")
		}
		if auth.Key.GetKey().GetPrivate() != "" {
			privateKey, err := crypto.TextDecrypt(auth.Key.GetKey().GetPrivate(), aesKey)
			if err != nil {
				return nil, fmt.Errorf("decrypt scheme private key: %w", err)
			}
			auth.Key.GetKey().Private = privateKey
		}
		if auth.Key.GetKey().GetPassphrase() != "" {
			passphrase, err := crypto.TextDecrypt(auth.Key.GetKey().GetPassphrase(), aesKey)
			if err != nil {
				return nil, fmt.Errorf("decrypt scheme key passphrase: %w", err)
			}
			auth.Key.GetKey().Passphrase = passphrase
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
func newSession(ctx context.Context, scheme *schemepb.Scheme_Response, actx *authContext) (*session, error) {
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
