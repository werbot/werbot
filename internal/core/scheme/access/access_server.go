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
	keymessage "github.com/werbot/werbot/internal/core/key/proto/message"
	keyenum "github.com/werbot/werbot/internal/core/key/proto/enum"
	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	schemeauthpb "github.com/werbot/werbot/internal/core/scheme/proto/auth"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/crypto"
)

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

	if s.sh.Update != nil {
		if schemeType != s.schemeType {
			errGRPC := status.Error(codes.InvalidArgument, trace.MsgSchemeDiffers)
			return nil, trace.Error(errGRPC, s.sh.Log, nil)
		}

		var oldAccess schemeaccesspb.AccessScheme_Server_SSH
		if err := protojson.Unmarshal(s.schemeByte, &oldAccess); err != nil {
			return nil, trace.Error(err, s.sh.Log, nil)
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
		password := accessTmp.Password.GetPassword()
		accessTmp.Password.Password = crypto.TextEncrypt(password, internal.GetString("SECURITY_AES_KEY", "3D6A619811A17396E45D514695DCDA3A"))

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

		schemeKey.Private = crypto.TextEncrypt(schemeKey.GetPrivate(), internal.GetString("SECURITY_AES_KEY", "3D6A619811A17396E45D514695DCDA3A"))
		schemeKey.Passphrase = crypto.TextEncrypt(schemeKey.GetPassphrase(), internal.GetString("SECURITY_AES_KEY", "3D6A619811A17396E45D514695DCDA3A"))
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
