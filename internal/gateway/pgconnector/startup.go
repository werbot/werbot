package pgconnector

import (
	"bufio"
	"cmp"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgproto3"
	"google.golang.org/protobuf/encoding/protojson"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/connector/postgres"
	"github.com/werbot/werbot/internal/gateway"
)

// Startup packet codes and limits.
const (
	codeCancel     = 80877102
	codeSSL        = 80877103
	codeGSSENC     = 80877104
	maxStartup     = 10000 // bytes of a StartupMessage, length word included
	maxTLSRequests = 2     // SSLRequest/GSSENCRequest answered N before the start
	cancelLen      = 16    // a CancelRequest with a 4-byte secret key
)

var errUnsupported = fmt.Errorf("%w: unsupported startup packet", gateway.ErrDenied)

// PreSession reads the start of a client: encryption requests get N, a
// CancelRequest is served through the gateway's cancel keys and ends the
// stream, a StartupMessage is checked against the resource (no replication,
// a database within allowed_databases) before any session exists.
func (Connector) PreSession(ctx context.Context, r gateway.Resource, peek func(int) ([]byte, error), conn io.ReadWriteCloser) (gateway.PreSessionResult, error) {
	p, err := nextPacket(peek, func() error { _, err := conn.Write([]byte{'N'}); return err })
	if err != nil {
		return gateway.PreSessionResult{}, err
	}
	if p.code == codeCancel {
		cancelRequest(ctx, r, peek, p)
		return gateway.PreSessionResult{Handled: true}, nil
	}
	m, err := startupMessage(peek, p)
	if err != nil {
		return gateway.PreSessionResult{}, err
	}
	cfg, err := resourceConfig(r)
	if err != nil {
		return gateway.PreSessionResult{}, err
	}
	db, refusal := admit(cfg, m.Parameters)
	if refusal != "" {
		return gateway.PreSessionResult{}, refuse(conn, refusal)
	}
	return gateway.PreSessionResult{DBName: db}, nil
}

// packet is the header of the first startup packet after the encryption
// requests, off bytes into the stream.
type packet struct {
	off, size int
	code      uint32
}

// nextPacket walks the encryption requests at the front of a stream, calling
// deny for each, and returns the header of the packet after them.
func nextPacket(peek func(int) ([]byte, error), deny func() error) (packet, error) {
	for off, n := 0, 0; ; off, n = off+8, n+1 {
		h, err := peek(off + 8)
		if err != nil {
			return packet{}, err
		}
		p := packet{off: off, size: int(binary.BigEndian.Uint32(h[off:])), code: binary.BigEndian.Uint32(h[off+4:])}
		if p.code != codeSSL && p.code != codeGSSENC {
			return p, nil
		}
		if p.size != 8 || n == maxTLSRequests {
			return packet{}, errUnsupported
		}
		if err := deny(); err != nil {
			return packet{}, err
		}
	}
}

// startupMessage decodes the StartupMessage (protocol 3.0 or 3.2) of p.
func startupMessage(peek func(int) ([]byte, error), p packet) (*pgproto3.StartupMessage, error) {
	if (p.code != pgproto3.ProtocolVersion30 && p.code != pgproto3.ProtocolVersion32) || p.size < 8 || p.size > maxStartup {
		return nil, errUnsupported
	}
	b, err := peek(p.off + p.size)
	if err != nil {
		return nil, err
	}
	var m pgproto3.StartupMessage
	if m.Decode(b[p.off+4:p.off+p.size]) != nil {
		return nil, errUnsupported
	}
	return &m, nil
}

// readStartup consumes the start of the stream PreSession already answered
// and returns its StartupMessage.
func readStartup(cr *bufio.Reader) (*pgproto3.StartupMessage, error) {
	p, err := nextPacket(cr.Peek, func() error { return nil })
	if err != nil {
		return nil, err
	}
	m, err := startupMessage(cr.Peek, p)
	if err != nil {
		return nil, err
	}
	_, err = cr.Discard(p.off + p.size)
	return m, err
}

// cancelRequest cancels the session of the key in a 16-byte CancelRequest
// when it belongs to r; any other cancel request cancels nothing.
func cancelRequest(ctx context.Context, r gateway.Resource, peek func(int) ([]byte, error), p packet) {
	if p.size != cancelLen {
		return
	}
	b, err := peek(p.off + cancelLen)
	if err != nil {
		return
	}
	key := uint64(binary.BigEndian.Uint32(b[p.off+8:]))<<32 | uint64(binary.BigEndian.Uint32(b[p.off+12:]))
	cancels.Cancel(ctx, key, r.ID)
}

// resourceConfig is the validated config of a postgres resource.
func resourceConfig(r gateway.Resource) (*postgresv1.Config, error) {
	var cfg postgresv1.Config
	if err := protojson.Unmarshal(r.Config, &cfg); err != nil {
		return nil, fmt.Errorf("postgres: resource config: %w", err)
	}
	if err := postgres.Validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// admit checks the start parameters against cfg: it returns the database or
// the client text of a refusal.
func admit(cfg *postgresv1.Config, params map[string]string) (db, refusal string) {
	switch strings.ToLower(params["replication"]) {
	case "", "false", "off", "no", "0":
	default:
		return "", "werbot: replication connections are not allowed"
	}
	db, ok := chooseDB(cfg, params)
	if !ok {
		return "", `werbot: database "` + db + `" is not allowed`
	}
	return db, ""
}

// chooseDB is the database of a start ("" is the resource's) and whether
// allowed_databases admits it; an empty list admits any.
func chooseDB(cfg *postgresv1.Config, params map[string]string) (string, bool) {
	db := cmp.Or(params["database"], cfg.GetDatabase())
	allowed := cfg.GetAllowedDatabases()
	return db, len(allowed) == 0 || slices.ContainsFunc(allowed, func(g string) bool {
		ok, _ := path.Match(g, db)
		return ok
	})
}

// refuse sends a client the FATAL 42501 text of a refused start.
func refuse(w io.Writer, text string) error {
	sendFatal(w, &pgproto3.ErrorResponse{Severity: "FATAL", SeverityUnlocalized: "FATAL", Code: "42501", Message: text})
	return fmt.Errorf("%w: %s", gateway.ErrDenied, strings.TrimPrefix(text, "werbot: "))
}

// sendFatal writes e; the stream ends right after, so a write error is moot.
func sendFatal(w io.Writer, e *pgproto3.ErrorResponse) {
	b, _ := e.Encode(nil)
	_, _ = w.Write(b)
}

// handshake is what the client gets instead of the database's login: a
// protocol downgrade when it asked for more than 3.0, AuthenticationOk, the
// database's parameters in key order, the gateway's cancel key and the
// database's transaction status.
func handshake(m *pgproto3.StartupMessage, params map[string]string, key uint64, txStatus byte) []byte {
	var b []byte
	var opts []string
	for k := range m.Parameters {
		if strings.HasPrefix(k, "_pq_.") {
			opts = append(opts, k)
		}
	}
	if m.ProtocolVersion != pgproto3.ProtocolVersion30 || len(opts) > 0 {
		slices.Sort(opts)
		b, _ = (&pgproto3.NegotiateProtocolVersion{NewestMinorProtocol: 0, UnrecognizedOptions: opts}).Encode(b)
	}
	b, _ = (&pgproto3.AuthenticationOk{}).Encode(b)
	for _, k := range slices.Sorted(maps.Keys(params)) {
		b, _ = (&pgproto3.ParameterStatus{Name: k, Value: params[k]}).Encode(b)
	}
	k := binary.BigEndian.AppendUint64(nil, key) // ProcessID, then a 4-byte SecretKey
	b, _ = (&pgproto3.BackendKeyData{ProcessID: binary.BigEndian.Uint32(k), SecretKey: k[4:]}).Encode(b)
	b, _ = (&pgproto3.ReadyForQuery{TxStatus: txStatus}).Encode(b)
	return b
}
