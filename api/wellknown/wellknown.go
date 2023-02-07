package wellknown

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/pkg/webutil"
)

// JWKSResponse is ...
type JWKSResponse struct {
	// Keys is a list of public keys in JWK format.
	Keys []jwt.JWK `json:"keys"`
}

// @Summary      Show jwks information
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200 {object} JWKSResponse{}
// @Router       /.well-known/jwks.json [get]
func (h *Handler) jwks(c *fiber.Ctx) error {
	jwks := new(JWKSResponse)

	// TODO: show all keys
	jwk, err := jwt.MarshalJWK(internal.GetByteFromFile("JWT_PUBLIC_KEY", "./jwt_public.key"))
	jwks.Keys = append(jwks.Keys, jwk)

	if err != nil {
		return webutil.StatusBadRequest(c, "", jwt.JWK{})
	}

	return webutil.StatusOK(c, "", jwks)
}
