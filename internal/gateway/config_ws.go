package gateway

import (
	"errors"
	"flag"

	"github.com/werbot/werbot/internal/platform/config"
)

const (
	envWSAddr         = "WERBOT_GATEWAY_WS_ADDR"
	envWSTLSCert      = "WERBOT_GATEWAY_WS_TLS_CERT"
	envWSTLSKey       = "WERBOT_GATEWAY_WS_TLS_KEY"
	envTrustedProxies = "WERBOT_GATEWAY_TRUSTED_PROXIES"
)

// wsFlags registers the WebSocket input flags on fs. The returned pointer holds the
// raw --trusted-proxies value, which checkWS parses.
func (c *Config) wsFlags(fs *flag.FlagSet, env map[string]string) (trusted *string) {
	fs.StringVar(&c.WSAddr, "ws-addr", env[envWSAddr], "WebSocket input listen address; empty disables")
	fs.StringVar(&c.WSTLSCert, "ws-tls-cert", env[envWSTLSCert], "WebSocket input TLS certificate PEM file")
	fs.StringVar(&c.WSTLSKey, "ws-tls-key", env[envWSTLSKey], "WebSocket input TLS key PEM file")
	return fs.String("trusted-proxies", env[envTrustedProxies], "CIDRs allowed to set X-Forwarded-For on the WebSocket input, comma-separated")
}

// checkWS requires the TLS certificate and key together and parses the trusted proxies.
// The PEM files are read when the listener starts, not here.
func (c *Config) checkWS(trusted string) (err error) {
	if (c.WSTLSCert == "") != (c.WSTLSKey == "") {
		return errors.New(envWSTLSCert + " and " + envWSTLSKey + " must be set together")
	}
	c.TrustedProxies, err = config.ParsePrefixes(envTrustedProxies, trusted)
	return err
}
