package access

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/werbot/werbot/internal"
	keyenum "github.com/werbot/werbot/internal/core/key/proto/enum"
	keymessage "github.com/werbot/werbot/internal/core/key/proto/message"
	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	schemeauthpb "github.com/werbot/werbot/internal/core/scheme/proto/auth"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/crypto"
)

// encryptSecret encrypts a scheme secret with the configured AES key.
// Empty secrets stay empty; an unset or invalid key is a configuration error.
func encryptSecret(secret string) (string, error) {
	if secret == "" {
		return "", nil
	}

	aesKey := internal.GetString("SECURITY_AES_KEY", "")
	if !crypto.CheckAESKey(aesKey) {
		return "", status.Error(codes.FailedPrecondition, "SECURITY_AES_KEY must be a hex encoded 16, 24 or 32 byte AES key")
	}

	return crypto.TextEncrypt(secret, aesKey)
}

func NewServerTCP() proto.Message { return &schemeaccesspb.AccessScheme_Server_TCP{} }
func SetServerTCP(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_ServerTcp{ServerTcp: msg.(*schemeaccesspb.AccessScheme_Server_TCP)}
}

func NewServerUDP() proto.Message { return &schemeaccesspb.AccessScheme_Server_UDP{} }
func SetServerUDP(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_ServerUdp{ServerUdp: msg.(*schemeaccesspb.AccessScheme_Server_UDP)}
}

func NewServerSSH() proto.Message { return &schemeaccesspb.AccessScheme_Server_SSH{} }
func SetServerSSH(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_ServerSsh{ServerSsh: msg.(*schemeaccesspb.AccessScheme_Server_SSH)}
}

func NewServerTelnet() proto.Message { return &schemeaccesspb.AccessScheme_Server_Telnet{} }
func SetServerTelnet(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_ServerTelnet{ServerTelnet: msg.(*schemeaccesspb.AccessScheme_Server_Telnet)}
}

func (s *handler) handleServerTCP(in *schemeaccesspb.AccessScheme_ServerTcp) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_server_tcp
	newAccess := in.ServerTcp
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Server TCP #%s",
	}, nil
}

func (s *handler) handleServerUDP(in *schemeaccesspb.AccessScheme_ServerUdp) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_server_udp
	newAccess := in.ServerUdp
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Server UDP #%s",
	}, nil
}

func (s *handler) handleServerSSH(in *schemeaccesspb.AccessScheme_ServerSsh) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_server_ssh
	newAccess := in.ServerSsh

	var oldPassword string
	if s.sh.Update != nil {
		if schemeType != s.schemeType {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgSchemeDiffers)
			return nil, trace.Error(errGRPC, s.sh.Log, nil)
		}

		var oldAccess schemeaccesspb.AccessScheme_Server_SSH
		if err := protojson.Unmarshal(s.schemeByte, &oldAccess); err != nil {
			return nil, trace.Error(err, s.sh.Log, nil)
		}
		if oldPw, ok := oldAccess.GetAccess().(*schemeaccesspb.AccessScheme_Server_SSH_Password); ok {
			oldPassword = oldPw.Password.GetPassword()
		}

		// check auth methods
		oldAuthMethod := reflect.TypeOf(oldAccess.GetAccess()).Elem().Name()
		newAuthMethod := reflect.TypeOf(newAccess.GetAccess()).Elem().Name()
		if oldAuthMethod != newAuthMethod {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgAuthMethodDoesNotMatch)
			return nil, trace.Error(errGRPC, s.sh.Log, nil)
		}

		// replace alias
		newAccess.Alias = oldAccess.GetAlias()
		// update address
		if newAccess.GetAddress() == "" {
			newAccess.Address = oldAccess.GetAddress()
		}

		// update port
		if newAccess.GetPort() == 0 {
			newAccess.Port = oldAccess.GetPort()
		}
	} else {
		newAccess.Alias = s.alias
	}

	switch accessTmp := newAccess.GetAccess().(type) {
	case *schemeaccesspb.AccessScheme_Server_SSH_Password:
		// Editing address/port or login keeps the existing encrypted password
		// when the client does not send a new one.
		if s.sh.Update != nil && accessTmp.Password.GetPassword() == "" && oldPassword != "" {
			accessTmp.Password.Password = oldPassword
			break
		}
		if accessTmp.Password.GetPassword() == "" {
			errGRPC := status.Error(codes.InvalidArgument, "password is required")
			return nil, trace.Error(errGRPC, s.sh.Log, nil)
		}
		password, err := encryptSecret(accessTmp.Password.GetPassword())
		if err != nil {
			return nil, trace.Error(err, s.sh.Log, nil)
		}
		accessTmp.Password.Password = password

	case *schemeaccesspb.AccessScheme_Server_SSH_Agent:
		// TODO add agent access
		fmt.Print("agent")

	case *schemeaccesspb.AccessScheme_Server_SSH_Key:
		schemeKey := &keymessage.SchemeKey{}
		tmpKeyID := newAccess.GetKey().GetKeyId()

		if tmpKeyID == "00000000-0000-0000-0000-000000000000" {
			newKeySSH, err := crypto.NewSSHKey(keyenum.KeyType_ed25519.String())
			if err != nil {
				return nil, trace.Error(err, s.sh.Log, trace.MsgFailedCreatingSSHKey)
			}
			schemeKey.Public = string(newKeySSH.PublicKey)
			schemeKey.Private = string(newKeySSH.PrivateKey)
			schemeKey.FingerPrint = newKeySSH.FingerPrint
			schemeKey.Passphrase = newKeySSH.Passphrase
		} else {
			tmpKey, err := s.sh.Redis.Client.Get(context.Background(), fmt.Sprintf("tmp_key_ssh:%s", tmpKeyID)).Result()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					errGRPC := status.Error(codes.NotFound, trace.MsgKeyNotFound)
					return nil, trace.Error(errGRPC, s.sh.Log, nil)
				}
				return nil, trace.Error(err, s.sh.Log, nil)
			}
			s.sh.Redis.Client.Del(context.Background(), fmt.Sprintf("tmp_key_ssh:%s", tmpKeyID))

			if err := protojson.Unmarshal([]byte(tmpKey), schemeKey); err != nil {
				return nil, trace.Error(err, s.sh.Log, trace.MsgStructureIsBroken)
			}
		}

		encryptedPrivate, err := encryptSecret(schemeKey.GetPrivate())
		if err != nil {
			return nil, trace.Error(err, s.sh.Log, nil)
		}
		schemeKey.Private = encryptedPrivate

		encryptedPassphrase, err := encryptSecret(schemeKey.GetPassphrase())
		if err != nil {
			return nil, trace.Error(err, s.sh.Log, nil)
		}
		schemeKey.Passphrase = encryptedPassphrase

		accessTmp.Key.Access = &schemeauthpb.Auth_Key_Key{
			Key: schemeKey,
		}
	}

	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Server SSH #%s",
	}, nil
}

func (s *handler) handleServerTelnet(in *schemeaccesspb.AccessScheme_ServerTelnet) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_server_telnet
	newAccess := in.ServerTelnet
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Server Telnet #%s",
	}, nil
}
