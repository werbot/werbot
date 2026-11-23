package tlsca

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestServerCN(t *testing.T) {
	id := uuid.New()
	if got := ServerCN(id); got != "server:"+id.String() {
		t.Fatalf("ServerCN = %q", got)
	}
	if got, err := ParseServerCN(ServerCN(id)); err != nil || got != id {
		t.Fatalf("round trip: %v %v", got, err)
	}
	for _, cn := range []string{
		"",
		"server",
		"server:x",
		"server:",
		"gateway:" + id.String() + ":" + id.String(),
		"server:" + id.String() + ":" + id.String(),
		"Server:" + id.String(),
		"server:{" + id.String() + "}",
		"server:" + strings.ToUpper(id.String()),
		"server:" + strings.ReplaceAll(id.String(), "-", ""),
		"server:urn:uuid:" + id.String(),
	} {
		if _, err := ParseServerCN(cn); err == nil {
			t.Errorf("ParseServerCN(%q) must fail", cn)
		}
	}
}
