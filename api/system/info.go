package system

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/gofiber/fiber/v2"

	systempb "github.com/werbot/werbot/internal/core/system/proto/system"
	"github.com/werbot/werbot/internal/version"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Get update information
// @Description Retrieves the latest update information for core and web services based on Docker container labels and GitHub releases
// @Tags info
// @Produce json
// @Success 200 {object} webutil.HTTPResponse{result=map[string]map[string]any}
// @Failure 404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/system/update [get]
func (h *Handler) update(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	if !sessionData.IsProfileAdmin() {
		return webutil.StatusNotFound(c, nil)
	}

	dockerClient, err := client.NewClientWithOpts(client.WithHost("unix:///var/run/docker.sock"), client.WithAPIVersionNegotiation())
	if err != nil {
		return webutil.StatusInternalServerError(c, "Failed connect to docker")
	}
	defer dockerClient.Close()

	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "org.opencontainers.image.version")
	listContainers, err := dockerClient.ContainerList(context.Background(), container.ListOptions{
		All:     false,
		Filters: filterArgs,
	})
	if err != nil {
		return webutil.StatusInternalServerError(c, "Failed to show container list")
	}

	releaseURLs := map[string]string{
		"core": "https://api.github.com/repos/werbot/werbot/releases/latest",
		"web":  "https://api.github.com/repos/werbot/werbot/releases/latest",
	}

	releases := make(map[string]string)
	for key, url := range releaseURLs {
		release, err := webutil.GetLatestRelease(url)
		if err != nil {
			return webutil.StatusInternalServerError(c, "Failed to get latest version for "+key)
		}
		releases[key] = release[1:]
	}

	updates := make(map[string]map[string]any)
	for _, container := range listContainers {
		service := container.Labels["org.opencontainers.image.title"]
		version := container.Labels["org.opencontainers.image.version"]

		if service == "werbot.web" {
			updates["web"] = map[string]any{
				"installed": version,
				"actual":    releases["web"],
			}
		} else {
			updates[service] = map[string]any{
				"installed": version,
				"actual":    releases["core"],
			}
		}
	}

	return webutil.StatusOK(c, "Updates", updates)
}

// @Summary Get profile information
// @Description Retrieves profile metrics information
// @Tags info
// @Produce json
// @Param profile_id query string false "Profile UUID". Parameter Accessible with ROLE_ADMIN rights. 00000000-0000-0000-0000-000000000000 - show all
// @Success 200 {object} webutil.HTTPResponse{result=systempb.ProfileMetrics_Response}
// @Failure 404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/system/info [get]
func (h *Handler) info(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	request := &systempb.ProfileMetrics_Request{
		IsAdmin:   sessionData.IsProfileAdmin(),
		ProfileId: sessionData.ProfileID(c.Query("profile_id")),
	}

	rClient := systempb.NewSystemHandlersClient(h.Grpc)
	info, err := rClient.ProfileMetrics(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(info)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Short", result)
}

// @Summary Get API version
// @Description Retrieves the current version and commit hash of the API
// @Tags info
// @Produce json
// @Success 200 {object} webutil.HTTPResponse{result=map[string]string}
// @Router /v1/system/version [get]
func (h *Handler) version(c *fiber.Ctx) error {
	return webutil.StatusOK(c, "Apps version", map[string]string{
		"api": fmt.Sprintf("%s (%s)", version.Version(), version.Commit()),
	})
}
