package main

import (
	"strings"
	"testing"

	"github.com/werbot/werbot/internal/core/scheme/proto/access"
	schemeauthpb "github.com/werbot/werbot/internal/core/scheme/proto/auth"
	"github.com/werbot/werbot/pkg/crypto"
)

const authTestAESKey = "3D6A619811A17396E45D514695DCDA3A"

func encryptedSecret(t *testing.T, value string) string {
	t.Helper()
	encrypted, err := crypto.TextEncrypt(value, authTestAESKey)
	if err != nil {
		t.Fatalf("TextEncrypt: %v", err)
	}
	return encrypted
}

func TestDecryptSchemeAccessTelnet(t *testing.T) {
	t.Setenv("SECURITY_AES_KEY", authTestAESKey)

	scheme := &access.AccessScheme{
		Access: &access.AccessScheme_ServerTelnet{
			ServerTelnet: &access.AccessScheme_Server_Telnet{
				Access: &schemeauthpb.Auth_Password{
					Login:    "ubuntu1",
					Password: encryptedSecret(t, "secret123"),
				},
			},
		},
	}

	if err := decryptSchemeAccess(scheme, authTestAESKey); err != nil {
		t.Fatalf("decryptSchemeAccess: %v", err)
	}
	if got := scheme.GetServerTelnet().GetAccess().GetPassword(); got != "secret123" {
		t.Fatalf("password = %q, want %q", got, "secret123")
	}
}

func TestDecryptSchemeAccessTelnetMasked(t *testing.T) {
	t.Setenv("SECURITY_AES_KEY", authTestAESKey)

	scheme := &access.AccessScheme{
		Access: &access.AccessScheme_ServerTelnet{
			ServerTelnet: &access.AccessScheme_Server_Telnet{
				Access: &schemeauthpb.Auth_Password{Password: "***"},
			},
		},
	}

	if err := decryptSchemeAccess(scheme, authTestAESKey); err == nil {
		t.Fatal("decryptSchemeAccess() error = nil, want masked secret error")
	}
}

func TestSchemeAccessTelnetPlaintextPassword(t *testing.T) {
	scheme := &access.AccessScheme{
		Access: &access.AccessScheme_ServerTelnet{
			ServerTelnet: &access.AccessScheme_Server_Telnet{
				Access: &schemeauthpb.Auth_Password{Password: "ubuntu1"},
			},
		},
	}

	err := decryptSchemeAccess(scheme, authTestAESKey)
	if err == nil {
		t.Fatal("decryptSchemeAccess() error = nil, want decrypt error")
	}
	if !strings.Contains(err.Error(), "decrypt scheme password") {
		t.Fatalf("error = %q, want decrypt scheme password", err)
	}
}

func TestDecryptSchemeAccessUnsupported(t *testing.T) {
	scheme := &access.AccessScheme{
		Access: &access.AccessScheme_DesktopRdp{
			DesktopRdp: &access.AccessScheme_Desktop_RDP{},
		},
	}

	err := decryptSchemeAccess(scheme, authTestAESKey)
	if err == nil {
		t.Fatal("decryptSchemeAccess() error = nil, want unsupported type error")
	}
	if !strings.Contains(err.Error(), "scheme type desktop_rdp is not supported") {
		t.Fatalf("error = %q, want unsupported desktop_rdp", err)
	}
}

func TestDecryptSchemeAccessSSHPassword(t *testing.T) {
	t.Setenv("SECURITY_AES_KEY", authTestAESKey)

	scheme := &access.AccessScheme{
		Access: &access.AccessScheme_ServerSsh{
			ServerSsh: &access.AccessScheme_Server_SSH{
				Access: &access.AccessScheme_Server_SSH_Password{
					Password: &schemeauthpb.Auth_Password{
						Login:    "ubuntu1",
						Password: encryptedSecret(t, "secret123"),
					},
				},
			},
		},
	}

	if err := decryptSchemeAccess(scheme, authTestAESKey); err != nil {
		t.Fatalf("decryptSchemeAccess: %v", err)
	}
	if got := scheme.GetServerSsh().GetPassword().GetPassword(); got != "secret123" {
		t.Fatalf("password = %q, want %q", got, "secret123")
	}
}
