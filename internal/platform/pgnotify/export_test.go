package pgnotify

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// CloseConnWithTimeout runs closeConn with a temporary close timeout.
func CloseConnWithTimeout(ctx context.Context, conn *pgx.Conn, d time.Duration) {
	old := closeTimeout
	closeTimeout = d
	defer func() { closeTimeout = old }()
	closeConn(ctx, conn)
}
