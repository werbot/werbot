package hashchain_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/werbot/werbot/internal/platform/hashchain"
)

func TestChainVectors(t *testing.T) {
	assert.Equal(t, make([]byte, 32), hashchain.Genesis())
	h1 := hashchain.Next(hashchain.Genesis(), []byte("abc"))
	assert.Equal(t, "365aa7d8f7f9402c4b9434502b4cc89ddb09fe50d7cd95b493b834c62d5a5370", hex.EncodeToString(h1))
	h2 := hashchain.Next(h1, []byte("def"))
	assert.Equal(t, "532f0131db1635cb105c9753e41e7908599aa4dd144658c92e50b60113ad993a", hex.EncodeToString(h2))
}
