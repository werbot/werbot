// Package postgres registers the "postgres" resource type.
package postgres

import (
	"crypto/x509"
	"errors"
	"fmt"
	"path"
	"slices"

	"google.golang.org/protobuf/proto"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/connector"
)

// TypeName is the registry name of the type.
const TypeName = "postgres"

// Defaults and TLS modes of a *postgresv1.Config.
const (
	DefaultPort     = 5432
	DefaultDatabase = "postgres"
	SSLDisable      = "disable"
	SSLRequire      = "require"
	SSLVerifyFull   = "verify-full"
)

// Limits of a *postgresv1.Config: PostgreSQL identifiers are at most 63
// bytes (NAMEDATALEN - 1).
const (
	maxName      = 63
	maxAllowed   = 64
	maxCAPEMSize = 65536
)

func init() {
	connector.Register(connector.Type{
		Name:         TypeName,
		Category:     connector.CategoryDatabases,
		NewConfig:    func() proto.Message { return &postgresv1.Config{} },
		Validate:     Validate,
		Credentials:  []connector.CredentialKind{connector.CredentialPassword, connector.CredentialClientCert},
		Actions:      []string{"postgres.connect"},
		Level:        "L3",
		Kinds:        []connector.Kind{connector.KindStream},
		Recorded:     []connector.Kind{connector.KindStream},
		RecordFormat: connector.RecordQuerylog,
		Endpoint:     Endpoint,
	})
}

// Endpoint is the database host and port of a *postgresv1.Config.
func Endpoint(m proto.Message) connector.Endpoint {
	c := m.(*postgresv1.Config)
	port := uint16(c.GetPort()) //nolint:gosec // Validate caps the port at 65535
	if port == 0 {
		port = DefaultPort
	}
	return connector.Endpoint{Host: c.GetHost(), Ports: []connector.PortRange{{Port: port}}}
}

// Validate checks a *postgresv1.Config and sets its defaults: port 5432,
// database postgres, sslmode verify-full, record_params true. The error
// names the invalid field.
func Validate(m proto.Message) error {
	c, ok := m.(*postgresv1.Config)
	if !ok {
		return fmt.Errorf("postgres: unexpected config type %T", m)
	}
	if err := validateTarget(c); err != nil {
		return err
	}
	if err := validateAllowed(c.GetAllowedDatabases()); err != nil {
		return err
	}
	if err := validateTLS(c); err != nil {
		return err
	}
	if c.RecordParams == nil {
		c.RecordParams = proto.Bool(true)
	}
	return nil
}

// validateTarget checks host, port, database and username and sets the
// default port and database.
func validateTarget(c *postgresv1.Config) error {
	if !connector.ValidHost(c.GetHost()) {
		return errors.New("postgres: invalid host")
	}
	if c.GetPort() > 65535 {
		return errors.New("postgres: invalid port")
	}
	if c.GetPort() == 0 {
		c.Port = DefaultPort
	}
	if len(c.GetDatabase()) > maxName {
		return errors.New("postgres: invalid database")
	}
	if c.GetDatabase() == "" {
		c.Database = DefaultDatabase
	}
	if u := c.GetUsername(); u == "" || len(u) > maxName || !connector.ValidLogin(u) {
		return errors.New("postgres: invalid username")
	}
	return nil
}

// validateAllowed checks the database globs: at most 64, each 1–63 bytes
// and a valid path.Match pattern.
func validateAllowed(globs []string) error {
	if len(globs) > maxAllowed {
		return errors.New("postgres: invalid allowed_databases")
	}
	for _, g := range globs {
		if _, err := path.Match(g, ""); g == "" || len(g) > maxName || err != nil {
			return errors.New("postgres: invalid allowed_databases")
		}
	}
	return nil
}

// validateTLS checks sslmode and ca_pem and sets sslmode verify-full when
// it is empty.
func validateTLS(c *postgresv1.Config) error {
	if !slices.Contains([]string{"", SSLDisable, SSLRequire, SSLVerifyFull}, c.GetSslmode()) {
		return errors.New("postgres: invalid sslmode")
	}
	if c.GetSslmode() == "" {
		c.Sslmode = SSLVerifyFull
	}
	if pem := c.GetCaPem(); len(pem) > maxCAPEMSize || (pem != "" && !x509.NewCertPool().AppendCertsFromPEM([]byte(pem))) {
		return errors.New("postgres: invalid ca_pem")
	}
	return nil
}
