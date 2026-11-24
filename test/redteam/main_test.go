//go:build redteam

package redteam

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/werbot/werbot/test/cluster"
)

func TestMain(m *testing.M) {
	flag.Parse()
	ctx := context.Background()
	keep := fmt.Sprintf("stand left for inspection; remove it with: docker compose -p %s -f test/cluster/docker-compose.yaml down -v\n", cluster.Project())
	var err error
	if home, err = os.MkdirTemp("", "werbot-redteam-"); err != nil {
		fmt.Fprintf(os.Stderr, "red-team home: %v\n", err)
		os.Exit(1)
	}
	if st, err = cluster.Up(ctx, cluster.Options{}); err != nil {
		fmt.Fprintf(os.Stderr, "red-team stand: %v\n%s", err, keep)
		_ = os.RemoveAll(home)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(home) // alice's CLI profile: throwaway keys of this stand
	if code != 0 {
		fmt.Fprint(os.Stderr, keep)
		os.Exit(code)
	}
	if err := st.Down(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "red-team stand: %v\n", err)
		os.Exit(1)
	}
}
