package postgres

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/connector"
)

func TestPostgresType(t *testing.T) {
	ty, ok := connector.Lookup("postgres")
	require.True(t, ok)
	assert.Equal(t, connector.CategoryDatabases, ty.Category)
	assert.Equal(t, "L3", ty.Level)
	assert.Equal(t, []connector.Kind{connector.KindStream}, ty.Kinds)
	assert.Equal(t, []connector.Kind{connector.KindStream}, ty.Recorded)
	assert.Equal(t, connector.RecordQuerylog, ty.RecordFormat)
	assert.Equal(t, []connector.CredentialKind{connector.CredentialPassword, connector.CredentialClientCert}, ty.Credentials)
	assert.Equal(t, []string{"postgres.connect", "postgres.query"}, ty.Actions)
	assert.IsType(t, &postgresv1.Config{}, ty.NewConfig())
}

func TestValidateDefaults(t *testing.T) {
	c := &postgresv1.Config{Host: "db", Username: "app"}
	require.NoError(t, Validate(c))
	assert.EqualValues(t, DefaultPort, c.GetPort())
	assert.Equal(t, DefaultDatabase, c.GetDatabase())
	assert.Equal(t, sslVerifyFull, c.GetSslmode())
	require.NotNil(t, c.RecordParams, "record_params unset becomes set")
	assert.True(t, c.GetRecordParams(), "record_params unset means true")

	off := &postgresv1.Config{Host: "db", Username: "app", Port: 6432, Database: "app", Sslmode: sslVerifyCA, CaPem: testCA(t), RecordParams: proto.Bool(false)}
	require.NoError(t, Validate(off))
	assert.EqualValues(t, 6432, off.GetPort())
	assert.Equal(t, "app", off.GetDatabase())
	assert.Equal(t, sslVerifyCA, off.GetSslmode())
	assert.False(t, off.GetRecordParams(), "an explicit false stays")

	full := &postgresv1.Config{Host: "::1", Username: "app", Sslmode: sslDisable, AllowedDatabases: []string{"app_*", "reports"},
		CaPem: testCA(t)}
	require.NoError(t, Validate(full))
}

func TestValidateErrors(t *testing.T) {
	ok := func() *postgresv1.Config { return &postgresv1.Config{Host: "db", Username: "app"} }
	for name, tc := range map[string]struct {
		mod   func(*postgresv1.Config)
		field string
	}{
		"empty host":          {func(c *postgresv1.Config) { c.Host = "" }, "host"},
		"host with space":     {func(c *postgresv1.Config) { c.Host = "a b" }, "host"},
		"port 70000":          {func(c *postgresv1.Config) { c.Port = 70000 }, "port"},
		"long database":       {func(c *postgresv1.Config) { c.Database = strings.Repeat("d", 64) }, "database"},
		"sslmode prefer":      {func(c *postgresv1.Config) { c.Sslmode = "prefer" }, "sslmode"},
		"sslmode require":     {func(c *postgresv1.Config) { c.Sslmode = "require" }, "sslmode"},
		"verify-ca no ca_pem": {func(c *postgresv1.Config) { c.Sslmode = "verify-ca" }, "ca_pem"},
		"bad glob":            {func(c *postgresv1.Config) { c.AllowedDatabases = []string{"["} }, "allowed_databases"},
		"empty glob":          {func(c *postgresv1.Config) { c.AllowedDatabases = []string{""} }, "allowed_databases"},
		"long glob":           {func(c *postgresv1.Config) { c.AllowedDatabases = []string{strings.Repeat("g", 64)} }, "allowed_databases"},
		"too many globs":      {func(c *postgresv1.Config) { c.AllowedDatabases = make([]string, 65) }, "allowed_databases"},
		"junk ca_pem":         {func(c *postgresv1.Config) { c.CaPem = "junk" }, "ca_pem"},
		"long ca_pem":         {func(c *postgresv1.Config) { c.CaPem = strings.Repeat("c", 65537) }, "ca_pem"},
		"empty username":      {func(c *postgresv1.Config) { c.Username = "" }, "username"},
		"username 64 chars":   {func(c *postgresv1.Config) { c.Username = strings.Repeat("u", 64) }, "username"},
		"username with space": {func(c *postgresv1.Config) { c.Username = "a b" }, "username"},
		"bad secret glob":     {func(c *postgresv1.Config) { c.SecretTables = []string{"["} }, "secret_tables"},
		"65 secret globs":     {func(c *postgresv1.Config) { c.SecretTables = make([]string, 65) }, "secret_tables"},
		"agent username 64":   {func(c *postgresv1.Config) { c.AgentUsername = strings.Repeat("u", 64) }, "agent_username"},
		"agent username junk": {func(c *postgresv1.Config) { c.AgentUsername = "a b" }, "agent_username"},
	} {
		c := ok()
		tc.mod(c)
		err := Validate(c)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), tc.field, name)
	}
	assert.Error(t, Validate(wrapperspb.String("x")))
}

func TestPostgresQueryAction(t *testing.T) {
	ty, _ := connector.Lookup("postgres")
	a, ok := ty.QueryAction()
	assert.True(t, ok)
	assert.Equal(t, "postgres.query", a)
	ssh := connector.Type{Name: "ssh", Actions: []string{"ssh.connect", "ssh.exec"}}
	a, ok = ssh.QueryAction()
	assert.False(t, ok)
	assert.Empty(t, a)

	c := &postgresv1.Config{Host: "db", Username: "app", AgentUsername: "app_ro", SecretTables: []string{"billing.*", "public.secret_?"}}
	require.NoError(t, Validate(c), "agent_username and secret_tables are valid")
	c.AgentUsername = ""
	require.NoError(t, Validate(c), "agent_username may stay empty")
}

func TestEndpoint(t *testing.T) {
	ty, _ := connector.Lookup("postgres")
	e, ok, err := ty.EndpointOf([]byte(`{"host":"pg","username":"app"}`))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, connector.Endpoint{Host: "pg", Ports: []connector.PortRange{{Port: DefaultPort}}}, e)
	assert.True(t, e.Allow("pg", "pg", 5432))
	assert.False(t, e.Allow("pg", "pg", 5433))

	e = ty.Endpoint(&postgresv1.Config{Host: "pg", Port: 6432})
	assert.True(t, e.Allow("pg", "pg", 6432))
	assert.False(t, e.Allow("pg", "pg", 5432))
}

// testCA is a self-signed CA certificate in PEM.
func testCA(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test ca"}, NotBefore: time.Now(),
		NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}
