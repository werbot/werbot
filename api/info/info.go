package info

import (
	"context"
	"fmt"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
	infopb "github.com/werbot/werbot/internal/grpc/info/proto"
	"github.com/werbot/werbot/internal/version"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary      Installed and actual versions of components
// @Tags         info
// @Accept       json
// @Produce      json
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/update [get]
func (h *Handler) getUpdate(c *fiber.Ctx) error {
	userParameter := middleware.AuthUser(c)

	if !userParameter.IsUserAdmin() {
		return webutil.StatusNotFound(c, nil)
	}

	client, err := docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInternalServerError(c, "Failed connect to docker")
	}

	listContainers, err := client.ListContainers(docker.ListContainersOptions{
		All: false,
		Filters: map[string][]string{
			"label": {"org.opencontainers.image.version"},
		},
	})
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInternalServerError(c, "Failed to show container list")
	}

	coreRelease, err := webutil.GetLatestRelease("https://api.github.com/repos/werbot/werbot/releases/latest")
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInternalServerError(c, "Failed to get latest version for werbot")
	}

	webRelease, err := webutil.GetLatestRelease("https://api.github.com/repos/werbot/werbot/releases/latest")
	if err != nil {
		h.log.Error(err).Send()
		return webutil.StatusInternalServerError(c, "Failed to get latest version for web app")
	}

	updates := make(map[string]map[string]any)
	for _, image := range listContainers {
		service := image.Labels["org.opencontainers.image.title"]
		if service == "werbot.web" {
			updates["web"] = map[string]any{
				"installed": image.Labels["org.opencontainers.image.version"],
				"actual":    webRelease[1:],
			}
		} else {
			updates[service] = map[string]any{
				"installed": image.Labels["org.opencontainers.image.version"],
				"actual":    coreRelease[1:],
			}
		}
	}

	return webutil.StatusOK(c, "Updates", updates)
}

// @Summary      Unexpected error while getting info
// @Tags         info
// @Accept       json
// @Produce      json
// @Success      200         {object} webutil.HTTPResponse{data=infopb.UserMetrics_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/info [get]
func (h *Handler) getInfo(c *fiber.Ctx) error {
	request := &infopb.UserMetrics_Request{}

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, nil)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())
	userRole := userParameter.UserRole()

	if request.UserId == userParameter.OriginalUserID() {
		request.Role = userRole
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := infopb.NewInfoHandlersClient(h.Grpc)
	info, err := rClient.UserMetrics(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Short information", info)
}

// @Summary      Version API
// @Tags         info
// @Accept       json
// @Produce      json
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/version [get]
func (h *Handler) getVersion(c *fiber.Ctx) error {
	// userParameter := middleware.GetUserParametersFromCtx(c)
	// if userParameter.UserRole != pb_user.Role_admin {
	// 	return webutil.StatusNotFound(c, internal.ErrNotFound, nil)
	// }

	versions := make(map[string]string)
	versions["api"] = fmt.Sprintf("%s (%s)", version.Version(), version.Commit())

	return webutil.StatusOK(c, "Apps version", versions)
}
