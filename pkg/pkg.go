package pkg

import (
	// Register protovalidate's CEL constraints through its init function.
	_ "buf.build/go/protovalidate"
	// Register the pgx database/sql driver through its init function.
	_ "github.com/jackc/pgx/v5/stdlib"
)
