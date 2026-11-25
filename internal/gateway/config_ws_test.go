package gateway

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigWS(t *testing.T) {
	c, err := LoadConfig(nil, env(nil))
	require.NoError(t, err)
	require.Empty(t, c.WSAddr)
	require.Empty(t, c.WSTLSCert)
	require.Empty(t, c.WSTLSKey)
	require.Nil(t, c.TrustedProxies)

	c, err = LoadConfig([]string{"--ws-addr", ":9000"}, env(map[string]string{envWSAddr: ":8022"}))
	require.NoError(t, err)
	require.Equal(t, ":9000", c.WSAddr)
	c, err = LoadConfig(nil, env(map[string]string{envWSAddr: ":8022"}))
	require.NoError(t, err)
	require.Equal(t, ":8022", c.WSAddr)

	c, err = LoadConfig([]string{"--ws-tls-cert", "c.pem", "--ws-tls-key", "k.pem"}, env(nil))
	require.NoError(t, err)
	require.Equal(t, "c.pem", c.WSTLSCert)
	require.Equal(t, "k.pem", c.WSTLSKey)
}

func TestLoadConfigWSTLSTogether(t *testing.T) {
	const want = "WERBOT_GATEWAY_WS_TLS_CERT and WERBOT_GATEWAY_WS_TLS_KEY must be set together"
	_, err := LoadConfig(nil, env(map[string]string{envWSTLSCert: "c.pem"}))
	require.ErrorContains(t, err, want)
	_, err = LoadConfig([]string{"--ws-tls-key", "k.pem"}, env(nil))
	require.ErrorContains(t, err, want)
}

func TestLoadConfigTrustedProxies(t *testing.T) {
	c, err := LoadConfig(nil, env(map[string]string{envTrustedProxies: "10.0.0.0/8, 192.168.1.10/32"}))
	require.NoError(t, err)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("192.168.1.10/32")}, c.TrustedProxies)

	c, err = LoadConfig([]string{"--trusted-proxies", "172.16.0.0/12"}, env(map[string]string{envTrustedProxies: "10.0.0.0/8"}))
	require.NoError(t, err)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12")}, c.TrustedProxies)

	_, err = LoadConfig(nil, env(map[string]string{envTrustedProxies: "10.0.0.0/33"}))
	require.ErrorContains(t, err, envTrustedProxies)
}
