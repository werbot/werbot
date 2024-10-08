package scheme

import (
	"context"
	"fmt"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/werbot/werbot/api/auth"
	keypb "github.com/werbot/werbot/internal/grpc/key/proto/key"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/storage/redis"
)

const pathSchemes = "/v1/schemes"

func setupTest(t *testing.T) (*test.APIHandler, func(t *testing.T), map[string]string, map[string]string) {
	app, teardownTestCase := test.API(t)
	auth.New(app.Handler).Routes()
	New(app.Handler).Routes()
	app.AddRoute404()

	adminHeader, userHeader := app.TestUserAuth()

	return app, teardownTestCase, adminHeader, userHeader
}

func serverKeygen(rd *redis.Connect, uuid string, empty bool) func() {
	return func() {
		cacheKeyStr := fmt.Sprintf("tmp_key_ssh:%s", uuid)
		expiration := time.Duration(10 * float64(time.Second))

		if empty {
			_ = rd.Client.Set(context.Background(), cacheKeyStr, "{}", expiration)
		} else {
			newKeySSH, _ := crypto.NewSSHKey(keypb.KeyType_ed25519.String())
			schemeKey := &keypb.SchemeKey{
				Public:      string(newKeySSH.PublicKey),
				Private:     string(newKeySSH.PrivateKey),
				Passphrase:  newKeySSH.Passphrase,
				FingerPrint: newKeySSH.FingerPrint,
			}
			json, _ := protojson.Marshal(schemeKey)
			_ = rd.Client.Set(context.Background(), cacheKeyStr, string(json), expiration)
		}
	}
}
