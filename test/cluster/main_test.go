//go:build cluster

package cluster

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
)

// st is the stand shared by every scenario; scenarios never run in parallel.
var st *Stand

func TestMain(m *testing.M) {
	flag.Parse()
	ctx := context.Background()
	keep := fmt.Sprintf("stand left for inspection; remove it with: docker compose -p %s -f test/cluster/docker-compose.yaml down -v\n", Project())
	var err error
	if st, err = Up(ctx, Options{}); err != nil {
		fmt.Fprintf(os.Stderr, "cluster stand: %v\n%s", err, keep)
		os.Exit(1)
	}
	code := m.Run()
	if code != 0 {
		fmt.Fprint(os.Stderr, keep)
		os.Exit(code)
	}
	if err := st.Down(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "cluster stand: %v\n", err)
		os.Exit(1)
	}
}
