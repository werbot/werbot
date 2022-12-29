package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/api/proto/server"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/crypto"
	"github.com/werbot/werbot/pkg/strutil"
)

var authContextKey = sshportalContextKey("auth")

type sshportalContextKey string

type authContext struct {
	message         string
	authMethod      string
	authSuccess     bool
	uuid            string
	userID          string
	userName        string
	userFingerPrint string
	hostAddr        string
	hostID          string
	userAddr        string
	userCountry     string
	aesKey          string // TODO добавить в базу для пользователя уникальный AES ключ по которому будут шифроваться его данные
	serverList      []*server.Server_Response
	// serverList      map[int32]*server.Server
}

func (c authContext) userType() server.UserType {
	if c.userName == "healthcheck" {
		return server.UserType_HEALTHCHECK
	}

	if strings.HasPrefix(c.userName, "invite_") {
		return server.UserType_INVITE
	}

	nameArray := strutil.SplitNTrimmed(c.userName, "_", 3)
	if len(nameArray) == 3 && nameArray[2] != "" {
		return server.UserType_BASTION
	}
	return server.UserType_SHELL
}

func bastionClientConfig(ctx ssh.Context, host *server.Server_Response) (*gossh.ClientConfig, error) {
	actx := ctx.Value(authContextKey).(*authContext)

	host.Password = crypto.TextDecrypt(host.Password, actx.aesKey)
	host.KeyPrivate = crypto.TextDecrypt(host.KeyPrivate, actx.aesKey)
	host.KeyPassword = crypto.TextDecrypt(host.KeyPassword, actx.aesKey)

	clientConfig, err := clientConfig(host, dynamicHostKey(host))
	if err != nil {
		return nil, err
	}

	// TODO сюда можно добавить проверку ACL проверку по доступности прав

	return clientConfig, nil
}

func clientConfig(host *server.Server_Response, hk gossh.HostKeyCallback) (*gossh.ClientConfig, error) {
	auth := []gossh.AuthMethod{}

	if host.KeyPrivate == "" && host.Password == "" {
		return nil, errors.New("empty private key and password")
	}

	if host.Auth == "key" && host.KeyPrivate != "" {
		var signer gossh.Signer
		var err error
		// если у ключа есть пароль, использовать его
		if host.KeyPassword != "" {
			signer, err = gossh.ParsePrivateKeyWithPassphrase([]byte(host.KeyPrivate), []byte(host.KeyPassword))
		} else {
			signer, err = gossh.ParsePrivateKey([]byte(host.KeyPrivate))
		}
		if err != nil {
			return nil, errors.New("unable to parse private key")
		}
		auth = append(auth, gossh.PublicKeys(signer))
	}

	if host.Password != "" && host.Auth == "password" {
		auth = append(auth, gossh.Password(host.Password))
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
