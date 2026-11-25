package inventory

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/inventory/dbq"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/wsurl"
)

// hostKeyLine opens the sealed private host key and returns its public half
// in authorized_keys format without the newline.
func (g *Gateways) hostKeyLine(ciphertext []byte, keyID string) (string, error) {
	plain, err := g.box.Open(ciphertext, keyID)
	if err != nil {
		return "", err
	}
	signer, err := gossh.ParsePrivateKey(plain)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(gossh.MarshalAuthorizedKey(signer.PublicKey()))), nil
}

// PoolEndpoints returns every pool of the organization by id, without
// instances: clients need its SSHAddress and HostKey. Any member may read them.
func (g *Gateways) PoolEndpoints(ctx context.Context, orgID uuid.UUID) (map[uuid.UUID]Gateway, error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]Gateway{}
	err = dbx.InOrg(ctx, g.pool, orgID, p.UserID, func(tx pgx.Tx) error {
		rows, err := newStore(tx).ListGateways(ctx, orgID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if out[r.ID], err = g.toGateway(r); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// validAddress accepts "" (reset) or host:port with a port of 1 to 65535.
func validAddress(addr string) error {
	if addr == "" {
		return nil
	}
	host, port, err := net.SplitHostPort(addr)
	n, perr := strconv.Atoi(port)
	if err != nil || host == "" || strings.ContainsFunc(host, func(r rune) bool { return r <= ' ' }) || perr != nil || n < 1 || n > 65535 {
		return invalid("ssh_address must be host:port with a port of 1 to 65535")
	}
	return nil
}

// validateWSURL accepts "" (reset) or a wsurl.Valid URL whose scheme is
// lower-case, as clients match it.
func validateWSURL(s string) error {
	if s != "" && (!wsurl.Valid(s) || !strings.HasPrefix(s, "ws://") && !strings.HasPrefix(s, "wss://")) {
		return invalid("ws_url must be a ws:// or wss:// URL with a host, without userinfo, query or fragment")
	}
	return nil
}

// Update replaces both addresses clients dial for the pool; "" resets one. Owners and admins only.
func (g *Gateways) Update(ctx context.Context, id uuid.UUID, sshAddress, wsURL string) (Gateway, error) {
	var out Gateway
	err := g.admin(ctx, func(p principal.Principal, st Store, tx pgx.Tx) error {
		if err := errors.Join(validAddress(sshAddress), validateWSURL(wsURL)); err != nil {
			return err
		}
		n, err := st.UpdateGatewayAddresses(ctx, dbq.UpdateGatewayAddressesParams{ID: id, OrgID: p.OrgID, SshAddress: sshAddress, WsUrl: wsURL})
		if err != nil {
			return err
		}
		if n == 0 {
			return errGatewayNotFound
		}
		if err := userActor(ctx, p).audit(ctx, tx, p.OrgID, "gateway.update", "gateway", id, map[string]any{"ssh_address": sshAddress, "ws_url": wsURL}); err != nil {
			return err
		}
		rows, err := st.ListGateways(ctx, p.OrgID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.ID == id {
				out, err = g.toGateway(r)
				return err
			}
		}
		return errGatewayNotFound
	})
	return out, err
}

func (g *Gateways) toGateway(r dbq.ListGatewaysRow) (Gateway, error) {
	key, err := g.hostKeyLine(r.HostKeyCiphertext, r.HostKeyKeyID)
	if err != nil {
		return Gateway{}, err
	}
	return Gateway{ID: r.ID, OrgID: r.OrgID, Name: r.Name, CreatedAt: r.CreatedAt, SSHAddress: r.SshAddress, WSURL: r.WsUrl, HostKey: key}, nil
}
