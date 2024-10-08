package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal"
	schemepb "github.com/werbot/werbot/internal/grpc/scheme/proto/scheme"
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
	userFingerPrint string
	hostAddr        string
	hostID          string
	userAddr        string
	aesKey          string // TODO добавить в базу для пользователя уникальный AES ключ по которому будут шифроваться его данные
	serverList      []*schemepb.Scheme_Response
	// serverList      map[int32]*server.Server
}

func (c authContext) userType() schemepb.Type {
	if c.alias == "healthcheck" {
		return schemepb.Type_healthcheck
	}

	if strings.HasPrefix(c.alias, "invite_") {
		return schemepb.Type_invite
	}

	nameArray := strutil.SplitNTrimmed(c.alias, "_", 3)
	if len(nameArray) == 3 && nameArray[2] != "" {
		return schemepb.Type_bastion
	}
	return schemepb.Type_shell
}

func bastionClientConfig(ctx ssh.Context, host *schemepb.Scheme_Response) (*gossh.ClientConfig, error) {
	actx := ctx.Value(authContextKey).(*authContext)

	_ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := schemepb.NewSchemeHandlersClient(app.grpc)
	access, _ := rClient.SchemeAccess(_ctx, &schemepb.ServerAccess_Request{
		ProjectId: host.GetProjectId(),
		SchemeId:  host.GetSchemeId(),
	})

	accessDecrypt := &schemepb.SchemeAccess_Response{}

	switch access.GetAccess().(type) {
	case *schemepb.SchemeAccess_Response_Password:
		accessDecrypt.Access = &schemepb.SchemeAccess_Response_Password{
			Password: crypto.TextDecrypt(access.GetPassword(), actx.aesKey),
		}

	case *schemepb.SchemeAccess_Response_Key:
		accessDecrypt.Access = &schemepb.SchemeAccess_Response_Key{
			Key: &schemepb.AccessKey{
				Private:    crypto.TextDecrypt(access.GetKey().GetPrivate(), actx.aesKey),
				Passphrase: crypto.TextDecrypt(access.GetKey().GetPassphrase(), actx.aesKey),
			},
		}
	}

	clientConfig, err := clientConfig(host, accessDecrypt, dynamicHostKey(host))
	if err != nil {
		return nil, err
	}

	// TODO Here you can add ACL verification verification for the accessibility of rights

	return clientConfig, nil
}

func clientConfig(host *schemepb.Scheme_Response, access *schemepb.SchemeAccess_Response, hk gossh.HostKeyCallback) (*gossh.ClientConfig, error) {
	auth := []gossh.AuthMethod{}

	if access.GetKey().GetPrivate() == "" && access.GetPassword() == "" {
		return nil, errors.New("empty private key and password")
	}

	if host.Auth == schemepb.Auth_key && access.GetKey().GetPrivate() != "" {
		var signer gossh.Signer
		var err error
		// If the key has a password, use it
		if access.GetKey().GetPrivate() != "" {
			signer, err = gossh.ParsePrivateKeyWithPassphrase([]byte(access.GetKey().GetPrivate()), []byte(access.GetKey().GetPassphrase()))
		} else {
			signer, err = gossh.ParsePrivateKey([]byte(access.GetKey().GetPrivate()))
		}
		if err != nil {
			return nil, errors.New("unable to parse private key")
		}
		auth = append(auth, gossh.PublicKeys(signer))
	}

	if host.Auth == schemepb.Auth_password && access.GetPassword() != "" {
		auth = append(auth, gossh.Password(access.GetPassword()))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("no valid authentication method for host %q", host.Title)
	}

	return &gossh.ClientConfig{
		User:            host.Alias,
		HostKeyCallback: hk,
		Auth:            auth,
		Timeout:         time.Duration(internal.GetInt("SSHSERVER_IDLE_TIMEOUT", 300)) * time.Second,
	}, nil
}
