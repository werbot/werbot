package info

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/logger"
	"github.com/werbot/werbot/internal/utils/validate"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/internal/grpc/proto/info"
)

// @Summary      Installed and actual versions of components
// @Tags         info
// @Accept       json
// @Produce      json
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/update [get]
func (h *Handler) getUpdate(c *fiber.Ctx) error {
	userParameter := middleware.AuthUser(c)

	if !userParameter.IsUserAdmin() {
		return httputil.StatusNotFound(c, internal.ErrNotFound, nil)
	}

	client, err := docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		message := "Unable to connect to Docker"
		logger.OutErrorLog("docker", err, message)
		return httputil.InternalServerError(c, "Unable to get list of containers", nil)
	}

	listImages, err := client.ListImages(docker.ListImagesOptions{
		All: false,
		Filters: map[string][]string{
			"label": {"com.werbot.version"},
		},
	})
	if err != nil {
		message := "Unable to get list of containers"
		logger.OutErrorLog("docker", err, message)
		return httputil.InternalServerError(c, "Unable to get list of containers", nil)
	}

	urlVersion := fmt.Sprintf("%s/v1/update/version", internal.GetString("API_DSN", "https://api.werbot.com"))
	getVersionInfo, err := http.Get(urlVersion)
	if err != nil {
		message := "Error getting data for updates"
		logger.OutErrorLog("docker", err, message)
		return httputil.InternalServerError(c, "Error getting data for updates", nil)
	}
	defer getVersionInfo.Body.Close()

	data, _ := io.ReadAll(getVersionInfo.Body)
	updateList := httputil.HTTPResponse{}
	json.Unmarshal(data, &updateList)

	updateComponent := updateList.Result.(map[string]any)

	updates := make(map[string]map[string]any)
	var regService = regexp.MustCompile(".*/(.*):.*")
	for _, image := range listImages {
		service := regService.FindStringSubmatch(image.RepoTags[0])
		if service != nil {
			updates[service[1]] = map[string]any{
				"version": image.Labels["com.werbot.version"],
				"update":  updateComponent[service[1]],
			}
		}
	}

	return httputil.StatusOK(c, "Installed and actual versions of components", updates)
}

// @Summary      Unexpected error while getting info
// @Tags         info
// @Accept       json
// @Produce      json
// @Success      200         {object} httputil.HTTPResponse{data=pb.GetInfo_Response}
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/info [get]
func (h *Handler) getInfo(c *fiber.Ctx) error {
	input := new(pb.GetInfo_Request)
	request := new(pb.GetInfo_Request)
	c.QueryParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.ErrValidateBodyParams, err)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(input.GetUserId())
	userRole := userParameter.UserRole()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewInfoHandlersClient(h.grpc.Client)

	if request.UserId == userParameter.OriginalUserID() {
		request.Role = userRole
	}

	info, err := rClient.GetInfo(ctx, request)
	if err != nil {
		return httputil.ReturnGRPCError(c, err)
	}

	return httputil.StatusOK(c, "Short information", info)
}

// @Summary      Version API
// @Tags         info
// @Accept       json
// @Produce      json
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/version [get]
func (h *Handler) getVersion(c *fiber.Ctx) error {
	// userParameter := middleware.GetUserParametersFromCtx(c)
	// if userParameter.UserRole != pb_user.RoleUser_ADMIN {
	// 	return httputil.StatusNotFound(c, internal.ErrNotFound, nil)
	// }

	info := fmt.Sprintf("%s (%s)", internal.Version(), internal.Commit())
	return httputil.StatusOK(c, "Version API", info)
}
