package system

import (
	"fmt"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/gofiber/fiber/v2"

	systempb "github.com/werbot/werbot/internal/grpc/system/proto/system"
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
	sessionData := session.AuthUser(c)
	if !sessionData.IsUserAdmin() {
		return webutil.StatusNotFound(c, nil)
	}

	client, err := docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		return webutil.StatusInternalServerError(c, "Failed connect to docker")
	}

	listContainers, err := client.ListContainers(docker.ListContainersOptions{
		All: false,
		Filters: map[string][]string{
			"label": {"org.opencontainers.image.version"},
		},
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
	for _, image := range listContainers {
		service := image.Labels["org.opencontainers.image.title"]
		version := image.Labels["org.opencontainers.image.version"]

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

// @Summary Get user information
// @Description Retrieves user metrics information
// @Tags info
// @Produce json
// @Param user_id query string false "User UUID". Parameter Accessible with ROLE_ADMIN rights. 00000000-0000-0000-0000-000000000000 - show all
// @Success 200 {object} webutil.HTTPResponse{result=systempb.UserMetrics_Response}
// @Failure 404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/system/info [get]
func (h *Handler) info(c *fiber.Ctx) error {
	sessionData := session.AuthUser(c)
	request := &systempb.UserMetrics_Request{
		IsAdmin: sessionData.IsUserAdmin(),
		UserId:  sessionData.UserID(c.Query("user_id")),
	}

	rClient := systempb.NewSystemHandlersClient(h.Grpc)
	info, err := rClient.UserMetrics(c.UserContext(), request)
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
