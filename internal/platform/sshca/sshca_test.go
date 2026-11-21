package sshca

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

var testNow = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

func newSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func testIdentity() Identity {
	return Identity{OrgID: uuid.New(), UserID: uuid.New(), CLISessionID: uuid.New(), Login: "alice"}
}

func signTest(t *testing.T, id Identity) *ssh.Certificate {
	t.Helper()
	cert, err := Sign(newSigner(t), newSigner(t).PublicKey(), id, testNow, UserCertTTL)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func TestSignParseRoundTrip(t *testing.T) {
	agent, run := uuid.New(), uuid.New()
	withAgent := testIdentity()
	withAgent.AgentID, withAgent.RunID = &agent, &run

	for name, id := range map[string]Identity{"user": testIdentity(), "agent": withAgent} {
		t.Run(name, func(t *testing.T) {
			ca := newSigner(t)
			cert, err := Sign(ca, newSigner(t).PublicKey(), id, time.Now(), UserCertTTL)
			if err != nil {
				t.Fatal(err)
			}
			checker := ssh.CertChecker{}
			if err := checker.CheckCert(id.Login, cert); err != nil {
				t.Fatalf("CheckCert: %v", err)
			}
			if !reflect.DeepEqual(cert.SignatureKey.Marshal(), ca.PublicKey().Marshal()) {
				t.Fatal("cert not signed by the CA")
			}
			got, err := Parse(cert)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, id) {
				t.Fatalf("Parse = %+v, want %+v", got, id)
			}
		})
	}
}

func TestSignValidityWindow(t *testing.T) {
	id := testIdentity()
	cert := signTest(t, id)
	if cert.CertType != ssh.UserCert {
		t.Fatalf("CertType = %d", cert.CertType)
	}
	if cert.KeyId != id.UserID.String() {
		t.Fatalf("KeyId = %q", cert.KeyId)
	}
	if !slices.Equal(cert.ValidPrincipals, []string{"alice"}) {
		t.Fatalf("ValidPrincipals = %v", cert.ValidPrincipals)
	}
	if want := uint64(testNow.Add(-time.Minute).Unix()); cert.ValidAfter != want { //nolint:gosec // fixed 2026 time
		t.Fatalf("ValidAfter = %d, want %d", cert.ValidAfter, want)
	}
	if want := uint64(testNow.Add(time.Hour).Unix()); cert.ValidBefore != want { //nolint:gosec // fixed 2026 time
		t.Fatalf("ValidBefore = %d, want %d", cert.ValidBefore, want)
	}
	if other := signTest(t, id); cert.Serial == other.Serial {
		t.Fatalf("serial repeats: %d", cert.Serial)
	}
}

func TestSignExtensions(t *testing.T) {
	cert := signTest(t, testIdentity())
	var keys []string
	for k := range cert.Extensions {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	want := []string{"permit-port-forwarding", "permit-pty", ExtOrg, ExtSession, ExtUser}
	slices.Sort(want)
	if !slices.Equal(keys, want) {
		t.Fatalf("extensions = %v, want %v", keys, want)
	}
	if cert.Extensions["permit-pty"] != "" || cert.Extensions["permit-port-forwarding"] != "" {
		t.Fatal("permit-* extensions must have empty values")
	}
	if len(cert.CriticalOptions) != 0 {
		t.Fatalf("critical options = %v", cert.CriticalOptions)
	}
}

func TestParseRejects(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ssh.Certificate)
		want   string
	}{
		{"host cert", func(c *ssh.Certificate) { c.CertType = ssh.HostCert }, "sshca: not a user certificate"},
		{"missing org", func(c *ssh.Certificate) { delete(c.Extensions, ExtOrg) }, "sshca: missing extension " + ExtOrg},
		{"missing user", func(c *ssh.Certificate) { delete(c.Extensions, ExtUser) }, "sshca: missing extension " + ExtUser},
		{"missing session", func(c *ssh.Certificate) { delete(c.Extensions, ExtSession) }, "sshca: missing extension " + ExtSession},
		{"bad org uuid", func(c *ssh.Certificate) { c.Extensions[ExtOrg] = "not-a-uuid" }, "sshca: invalid extension " + ExtOrg},
		{"bad agent uuid", func(c *ssh.Certificate) { c.Extensions[ExtAgent] = "not-a-uuid" }, "sshca: invalid extension " + ExtAgent},
		{"bad run uuid", func(c *ssh.Certificate) { c.Extensions[ExtRun] = "not-a-uuid" }, "sshca: invalid extension " + ExtRun},
		{"two principals", func(c *ssh.Certificate) { c.ValidPrincipals = []string{"alice", "root"} }, "sshca: want exactly one principal"},
		{"no principals", func(c *ssh.Certificate) { c.ValidPrincipals = nil }, "sshca: want exactly one principal"},
		{"empty principal", func(c *ssh.Certificate) { c.ValidPrincipals = []string{""} }, "sshca: want exactly one principal"},
		{"key id mismatch", func(c *ssh.Certificate) { c.KeyId = uuid.NewString() }, "sshca: key id does not match " + ExtUser},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cert := signTest(t, testIdentity())
			tc.mutate(cert)
			if _, err := Parse(cert); err == nil || err.Error() != tc.want {
				t.Fatalf("Parse err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestOpenSSHReadsCert(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not in PATH")
	}
	path := filepath.Join(t.TempDir(), "id-cert.pub")
	if err := os.WriteFile(path, ssh.MarshalAuthorizedKey(signTest(t, testIdentity())), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("ssh-keygen", "-L", "-f", path).CombinedOutput() //nolint:gosec // fixed binary, path from t.TempDir
	if err != nil {
		t.Fatalf("ssh-keygen: %v\n%s", err, out)
	}
	for _, want := range []string{ExtOrg, "Type: ssh-ed25519-cert-v01@openssh.com user certificate"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("ssh-keygen output lacks %q:\n%s", want, out)
		}
	}
}
