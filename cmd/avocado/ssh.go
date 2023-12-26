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
	"github.com/werbot/werbot/internal/crypto"
	serverpb "github.com/werbot/werbot/internal/grpc/server/proto"
	"github.com/werbot/werbot/pkg/strutil"
)

var authContextKey = sshportalContextKey("auth")

type sshportalContextKey string

type authContext struct {
	message         string
	authMethod      string
	authSuccess     bool
	sessionID       string
	userID          string
	login           string
	userFingerPrint string
	hostAddr        string
	hostID          string
	userAddr        string
	aesKey          string // TODO добавить в базу для пользователя уникальный AES ключ по которому будут шифроваться его данные
	serverList      []*serverpb.Server_Response
	// serverList      map[int32]*server.Server
}

func (c authContext) userType() serverpb.Type {
	if c.login == "healthcheck" {
		return serverpb.Type_healthcheck
	}

	if strings.HasPrefix(c.login, "invite_") {
		return serverpb.Type_invite
	}

	nameArray := strutil.SplitNTrimmed(c.login, "_", 3)
	if len(nameArray) == 3 && nameArray[2] != "" {
		return serverpb.Type_bastion
	}
	return serverpb.Type_shell
}

func bastionClientConfig(ctx ssh.Context, host *serverpb.Server_Response) (*gossh.ClientConfig, error) {
	actx := ctx.Value(authContextKey).(*authContext)

	_ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(app.grpc)
	access, _ := rClient.ServerAccess(_ctx, &serverpb.ServerAccess_Request{
		ProjectId: host.GetProjectId(),
		ServerId:  host.GetServerId(),
	})

	accessDecrypt := new(serverpb.ServerAccess_Response)

	switch access.GetAccess().(type) {
	case *serverpb.ServerAccess_Response_Password:
		accessDecrypt.Access = &serverpb.ServerAccess_Response_Password{
			Password: crypto.TextDecrypt(access.GetPassword(), actx.aesKey),
		}

	case *serverpb.ServerAccess_Response_Key:
		accessDecrypt.Access = &serverpb.ServerAccess_Response_Key{
			Key: &serverpb.ServerAccess_Key{
				Private:  crypto.TextDecrypt(access.GetKey().GetPrivate(), actx.aesKey),
				Password: crypto.TextDecrypt(access.GetKey().GetPassword(), actx.aesKey),
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

func clientConfig(host *serverpb.Server_Response, access *serverpb.ServerAccess_Response, hk gossh.HostKeyCallback) (*gossh.ClientConfig, error) {
	auth := []gossh.AuthMethod{}

	if access.GetKey().GetPrivate() == "" && access.GetPassword() == "" {
		return nil, errors.New("empty private key and password")
	}

	if host.Auth == serverpb.Auth_key && access.GetKey().GetPrivate() != "" {
		var signer gossh.Signer
		var err error
		// If the key has a password, use it
		if access.GetKey().GetPrivate() != "" {
			signer, err = gossh.ParsePrivateKeyWithPassphrase([]byte(access.GetKey().GetPrivate()), []byte(access.GetKey().GetPassword()))
		} else {
			signer, err = gossh.ParsePrivateKey([]byte(access.GetKey().GetPrivate()))
		}
		if err != nil {
			return nil, errors.New("unable to parse private key")
		}
		auth = append(auth, gossh.PublicKeys(signer))
	}

	if host.Auth == serverpb.Auth_password && access.GetPassword() != "" {
		auth = append(auth, gossh.Password(access.GetPassword()))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("no valid authentication method for host %q", host.Title)
	}

	return &gossh.ClientConfig{
		User:            host.Login,
		HostKeyCallback: hk,
		Auth:            auth,
		Timeout:         time.Duration(internal.GetInt("SSHSERVER_IDLE_TIMEOUT", 300)) * time.Second,
	}, nil
}
