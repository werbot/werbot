package jwt

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/werbot/werbot/internal"

	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	"github.com/werbot/werbot/internal/storage/cache"
)

// Config is ...
type Config struct {
	Clock      time.Time
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey

	Tokens  Tokens                    `json:"tokens"`
	Context *accountpb.UserParameters `json:"context"`
}

// Tokens is ...
type Tokens struct {
	Access  string `json:"access_token,omitempty"`
	Refresh string `json:"refresh_token" form:"refresh_token"`
}

// New is ...
func New(context *accountpb.UserParameters) (*Config, error) {
	var err error
	config := new(Config)

	config.Clock = time.Now()
	config.Context = context

	if config.PrivateKey, err = PrivateKey(); err != nil {
		return nil, err
	}

	if config.PublicKey, err = PublicKey(); err != nil {
		return nil, err
	}

	config.Tokens.Access, err = config.createToken(internal.GetDuration("ACCESS_TOKEN_DURATION", "15m"), true)
	if err != nil {
		return nil, err
	}

	config.Tokens.Refresh, err = config.createToken(internal.GetDuration("REFRESH_TOKEN_DURATION", "168h"), false)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (d *Config) createToken(expire time.Duration, accessToken bool) (string, error) {
	iat := d.Clock
	exp := iat.Add(expire)

	claims := UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   d.Context.GetSub(),
			IssuedAt:  jwt.NewNumericDate(iat),
			NotBefore: jwt.NewNumericDate(iat),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}

	if accessToken {
		claims.User = accountpb.UserParameters{
			UserName: d.Context.GetUserName(),
			UserId:   d.Context.GetUserId(),
			Roles:    d.Context.GetRoles(),
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, &claims)
	newToken, err := token.SignedString(d.PrivateKey)
	if err != nil {
		return "", err
	}

	return newToken, nil
}

// ValidateToken is ...
func ValidateToken(cache cache.Cache, sub string) bool {
	key := fmt.Sprintf("ref_token::%s", sub)
	if _, err := cache.Get(key).Result(); err != nil {
		return false
	}
	return true
}

// AddToken is ...
func AddToken(cache cache.Cache, sub string, data any) bool {
	key := fmt.Sprintf("ref_token::%s", sub)
	if err := cache.Set(key, data, internal.GetDuration("REFRESH_TOKEN_DURATION", "168h")); err != nil {
		return false
	}
	return true
}

// DeleteToken is ...
func DeleteToken(cache cache.Cache, sub string) bool {
	key := fmt.Sprintf("ref_token::%s", sub)
	if _, err := cache.Delete(key); err != nil {
		return false
	}
	return true
}

// VerifyToken is ...
func VerifyToken(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
	}

	publicKey, err := PublicKey()
	if err != nil {
		return nil, err
	}

	return publicKey, nil
}

// CustomKeyFunc is ...
func CustomKeyFunc() jwt.Keyfunc {
	return func(t *jwt.Token) (any, error) {
		return VerifyToken(t)
	}
}
