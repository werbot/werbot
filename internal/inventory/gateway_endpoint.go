package inventory

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/inventory/dbq"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// PoolEndpoint is where clients reach a pool and the key they must see there.
type PoolEndpoint struct {
	SSHAddress string // host:port, "" until an admin sets it
	HostKey    string // authorized_keys line of the pool host key
}

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

// PoolEndpoints returns the endpoint of every pool of the organization; any member may read them.
func (g *Gateways) PoolEndpoints(ctx context.Context, orgID uuid.UUID) (map[uuid.UUID]PoolEndpoint, error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]PoolEndpoint{}
	err = dbx.InOrg(ctx, g.pool, orgID, p.UserID, func(tx pgx.Tx) error {
		rows, err := newStore(tx).ListGateways(ctx, orgID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			key, err := g.hostKeyLine(r.HostKeyCiphertext, r.HostKeyKeyID)
			if err != nil {
				return err
			}
			out[r.ID] = PoolEndpoint{SSHAddress: r.SshAddress, HostKey: key}
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

// Update sets the address clients dial for the pool; "" resets it. Owners and admins only.
func (g *Gateways) Update(ctx context.Context, id uuid.UUID, sshAddress string) (Gateway, error) {
	var out Gateway
	err := g.admin(ctx, func(p principal.Principal, st Store, tx pgx.Tx) error {
		if err := validAddress(sshAddress); err != nil {
			return err
		}
		n, err := st.UpdateGatewayAddress(ctx, dbq.UpdateGatewayAddressParams{ID: id, OrgID: p.OrgID, SshAddress: sshAddress})
		if err != nil {
			return err
		}
		if n == 0 {
			return errGatewayNotFound
		}
		if err := userActor(ctx, p).audit(ctx, tx, p.OrgID, "gateway.update", "gateway", id, map[string]any{"ssh_address": sshAddress}); err != nil {
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
	return Gateway{ID: r.ID, OrgID: r.OrgID, Name: r.Name, CreatedAt: r.CreatedAt, SSHAddress: r.SshAddress, HostKey: key}, nil
}
