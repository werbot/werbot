package jwt

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/werbot/werbot/internal"

	"github.com/werbot/werbot/internal/grpc"
	pb "github.com/werbot/werbot/internal/grpc/proto/user"
	"github.com/werbot/werbot/internal/storage/cache"
)

// Config is ...
type Config struct {
	IssuedAt   time.Time
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey

	Tokens  Tokens             `json:"tokens"`
	Context *pb.UserParameters `json:"context"`
}

// Tokens is ...
type Tokens struct {
	Access  string `json:"access_token,omitempty"`
	Refresh string `json:"refresh_token" form:"refresh_token"`
}

// New is ...
func New(context *pb.UserParameters) (*Config, error) {
	var err error
	config := new(Config)

	config.IssuedAt = time.Now()
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
	iat := d.IssuedAt
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
		claims.User = pb.UserParameters{
			UserName: d.Context.GetUserName(),
			UserId:   d.Context.GetUserId(),
			Roles:    d.Context.GetRoles(),
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	newToken, err := token.SignedString(d.PrivateKey)
	if err != nil {
		return "", err
	}

	return newToken, nil
}

// Verify is ...
func Verify(token *jwt.Token) (any, error) {
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
		return Verify(t)
	}
}

// ValidateToken is ...
func ValidateToken(cache cache.Cache, sub string) bool {
	key := fmt.Sprintf("ref_token::%s", sub)
	if _, err := cache.Get(key); err != nil {
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

// RefreshToken is ...
func RefreshToken(cache cache.Cache, grpc *grpc.ClientService, refresh string) (*Tokens, error) {
	t, err := jwt.Parse(refresh, Verify)
	if err != nil {
		return nil, errors.New("Token parsing error")
	}

	if _, ok := t.Claims.(jwt.Claims); !ok && !t.Valid {
		return nil, errors.New("Token regeneration error")
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if ok && t.Valid {
		sub := claims["sub"].(string)
		userID, err := cache.Get(fmt.Sprintf("ref_token::%s", sub))

		if !ValidateToken(cache, sub) {
			return nil, errors.New("Your token has been revoked")
		}
		DeleteToken(cache, sub)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		rClient := pb.NewUserHandlersClient(grpc.Client)

		user, _ := rClient.GetUser(ctx, &pb.GetUser_Request{
			UserId: userID,
		})

		newToken, err := New(&pb.UserParameters{
			UserName: "TODO",
			UserId:   user.GetUserId(),
			Roles:    user.GetRole(),
			Sub:      sub,
		})
		if err != nil {
			return nil, errors.New("Failed to create token")
		}

		AddToken(cache, sub, userID)

		return &Tokens{
			Access:  newToken.Tokens.Access,
			Refresh: newToken.Tokens.Refresh,
		}, nil
	}

	return nil, errors.New("Refresh expired")
}

// PublicKey is ...
func PublicKey() (*rsa.PublicKey, error) {
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(internal.GetByteFromFile("JWT_PUBLIC_KEY", "./jwt_public.key"))
	if err != nil {
		return nil, err
	}
	return publicKey, nil
}

// PrivateKey is ...
func PrivateKey() (*rsa.PrivateKey, error) {
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(internal.GetByteFromFile("JWT_PRIVATE_KEY", "./jwt_private.key"))
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}
