package httputil

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/werbot/werbot/internal/config"
)

/*
// Config is ...
type Config struct {
	AccessDuration  string `json:"access_duration"`
	RefreshDuration string `json:"refresh_duration"`

	AccessSecret  string `json:"access_secret"`
	RefreshSecret string `json:"refresh_secret"`
}
*/

// Tokens is ...
type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Detail is ...
type Detail struct {
	Tokens          Tokens        `json:"tokens"`
	AccessTokenExp  time.Duration `json:"access_token_exp"`
	RefreshTokenExp time.Duration `json:"refresh_token_exp"`
	Context         any           `json:"context"`
}

// RefreshToken is ...
type RefreshToken struct {
	Token string `json:"refresh_token" form:"refresh_token"`
}

// CreateToken is ...
func CreateToken(sub string, context any) (*Detail, error) {
	err := error(nil)
	detail := &Detail{
		AccessTokenExp:  config.GetDuration("ACCESS_TOKEN_DURATION", "15m"),
		RefreshTokenExp: config.GetDuration("REFRESH_TOKEN_DURATION", "168h"),
		Context:         context,
	}

	detail.Tokens.AccessToken, err = generate(sub, config.GetString("ACCESS_TOKEN_SECRET", "accessTokenSecret"), detail.AccessTokenExp, detail.Context)
	if err != nil {
		return nil, err
	}

	detail.Tokens.RefreshToken, err = generate(sub, config.GetString("REFRESH_TOKEN_SECRET", "refreshTokenSecret"), detail.RefreshTokenExp, nil)
	if err != nil {
		return nil, err
	}

	return detail, nil
}

// VerifyToken is ...
func VerifyToken(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}
	return []byte(config.GetString("REFRESH_TOKEN_SECRET", "refreshTokenSecret")), nil
}

func generate(sub, signed string, expire time.Duration, context any) (string, error) {
	exp := time.Now().Add(expire).Unix()

	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["sub"] = sub
	claims["iat"] = time.Now().Unix()
	claims["exp"] = exp
	claims["context"] = context

	newToken, err := token.SignedString([]byte(signed))
	if err != nil {
		return "", err
	}

	return newToken, nil
}
