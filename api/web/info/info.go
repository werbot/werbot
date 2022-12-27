package info

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/api/proto/info"
)

// @Summary      Installed and actual versions of components
// @Tags         info
// @Accept       json
// @Produce      json
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/update [get]
func (h *handler) getUpdate(c *fiber.Ctx) error {
	userParameter := middleware.AuthUser(c)

	if !userParameter.IsUserAdmin() {
		return httputil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	client, err := docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		h.log.Error(err).Send()
		return httputil.InternalServerError(c, msgFailedToShowContainerList, nil)
	}

	listImages, err := client.ListImages(docker.ListImagesOptions{
		All: false,
		Filters: map[string][]string{
			"label": {"com.werbot.version"},
		},
	})
	if err != nil {
		h.log.Error(err).Send()
		return httputil.InternalServerError(c, msgFailedToShowContainerList, nil)
	}

	urlVersion := fmt.Sprintf("%s/v1/update/version", internal.GetString("API_DSN", "https://api.werbot.com"))
	getVersionInfo, err := http.Get(urlVersion)
	if err != nil {
		h.log.Error(err).Send()
		return httputil.InternalServerError(c, msgFailedToGetDataForUpdates, nil)
	}
	if getVersionInfo.StatusCode > 200 {
		return httputil.StatusNotFound(c, msgFailedToConnectUpdateServer, nil)
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

	return httputil.StatusOK(c, msgCurrentVersions, updates)
}

// @Summary      Unexpected error while getting info
// @Tags         info
// @Accept       json
// @Produce      json
// @Success      200         {object} httputil.HTTPResponse{data=pb.UserStatistics_Response}
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/info [get]
func (h *handler) getInfo(c *fiber.Ctx) error {
	request := new(pb.UserStatistics_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UserStatistics_RequestMultiError) {
			e := err.(pb.UserStatistics_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())
	userRole := userParameter.UserRole()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewInfoHandlersClient(h.Grpc.Client)

	if request.UserId == userParameter.OriginalUserID() {
		request.Role = userRole
	}

	info, err := rClient.UserStatistics(ctx, request)
	if err != nil {
		return httputil.FromGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, msgShortInfo, info)
}

// @Summary      Version API
// @Tags         info
// @Accept       json
// @Produce      json
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/version [get]
func (h *handler) getVersion(c *fiber.Ctx) error {
	// userParameter := middleware.GetUserParametersFromCtx(c)
	// if userParameter.UserRole != pb_user.RoleUser_ADMIN {
	// 	return httputil.StatusNotFound(c, internal.ErrNotFound, nil)
	// }

	info := fmt.Sprintf("%s (%s)", internal.Version(), internal.Commit())
	return httputil.StatusOK(c, msgAPIVersion, info)
}
