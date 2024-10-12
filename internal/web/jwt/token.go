package jwt

import (
	"crypto/rsa"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/werbot/werbot/internal"
	profilepb "github.com/werbot/werbot/internal/core/profile/proto/profile"
	"github.com/werbot/werbot/pkg/storage/redis"
)

// Config holds the configuration for JWT token generation
type Config struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	Context    *profilepb.ProfileParameters
}

// New creates a new instance of Config with initialized keys
func New(context *profilepb.ProfileParameters) (*Config, error) {
	privateKey, err := PrivateKey()
	if err != nil {
		return nil, err
	}

	publicKey, err := PublicKey()
	if err != nil {
		return nil, err
	}

	return &Config{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Context:    context,
	}, nil
}

// PairTokens generates both an access token and a refresh token.
func (d *Config) PairTokens() (*profilepb.Token_Response, error) {
	accessToken, err := d.createToken(internal.GetDuration("ACCESS_TOKEN_DURATION", "15m"), true)
	if err != nil {
		return nil, err
	}

	refreshToken, err := d.createToken(internal.GetDuration("REFRESH_TOKEN_DURATION", "168h"), false)
	if err != nil {
		return nil, err
	}

	return &profilepb.Token_Response{
		Access:  accessToken,
		Refresh: refreshToken,
	}, nil
}

// createToken generates a JWT token with the given expiry duration and type (access or refresh).
func (d *Config) createToken(expire time.Duration, accessToken bool) (string, error) {
	now := time.Now()
	exp := now.Add(expire)

	claims := UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   d.Context.GetSessionId(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}

	if accessToken {
		claims.User = profilepb.ProfileParameters{
			Name:      d.Context.GetName(),
			ProfileId: d.Context.GetProfileId(),
			Roles:     d.Context.GetRoles(),
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, &claims)
	return token.SignedString(d.PrivateKey)
}

type SessionInfo struct {
	ProfileID string `json:"profile_id" redis:"profile_id"`
}

// CacheFind search the entry with the specified key prefix and UUID from the cache.
func CacheGet(redis *redis.Connect, keyPrefix, sessionID string) (*SessionInfo, error) {
	key := keyPrefix + ":" + sessionID

	var data SessionInfo
	redis.Client.HGetAll(redis.Ctx, key).Scan(&data)

	//dataCache, err := redis.Client.Get(redis.Ctx, key).Result()
	//if err != nil {
	//	return nil, err
	//}
	//
	//var data SessionInfo
	//if err := json.Unmarshal([]byte(dataCache), &data); err != nil {
	//	return nil, err
	//}
	return &data, nil
}

// CacheAdd adds the given data to the cache with the specified key prefix and UUID.
func CacheAdd(redis *redis.Connect, keyPrefix, sessionID string, data any) {
	duration := internal.GetDuration(strings.ToUpper(keyPrefix)+"_DURATION", "168h")
	key := keyPrefix + ":" + sessionID

	redis.Client.HSet(redis.Ctx, key, data)
	redis.Client.Expire(redis.Ctx, key, duration)

	// jsonData, _ := json.Marshal(data)
	// err := redis.Client.Set(redis.Ctx, key, jsonData, duration)
	// return err == nil
}

// CacheDelete removes the entry with the specified key prefix and UUID from the cache.
func CacheDelete(redis *redis.Connect, keyPrefix, sessionID string) bool {
	key := keyPrefix + ":" + sessionID
	return redis.Client.Del(redis.Ctx, key) == nil
}

// -----------

func VerifyToken(token string) bool {
	v, err := jwt.Parse(token, verifyToken)
	if err != nil {
		return false
	}

	return v.Valid
}

// VerifyToken is ...
func verifyToken(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, errors.New("unexpected signing method")
	}

	return PublicKey()
}

// CustomKeyFunc is ...
func CustomKeyFunc() jwt.Keyfunc {
	return func(t *jwt.Token) (any, error) {
		return verifyToken(t)
	}
}
