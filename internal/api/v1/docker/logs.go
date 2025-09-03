package dockerapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	apitypes "github.com/yusing/go-proxy/internal/api/types"
	"github.com/yusing/go-proxy/internal/docker"
	"github.com/yusing/go-proxy/internal/net/gphttp/websocket"
	"github.com/yusing/go-proxy/internal/task"
)

type LogsQueryParams struct {
	Stdout bool   `form:"stdout,default=true"`
	Stderr bool   `form:"stderr,default=true"`
	Since  string `form:"from"`
	Until  string `form:"to"`
	Levels string `form:"levels"`
} //	@name	LogsQueryParams

// @x-id				"logs"
// @BasePath		/api/v1
// @Summary		Get docker container logs
// @Description	Get docker container logs by container id
// @Tags			docker,websocket
// @Accept			json
// @Produce		json
// @Param			id	path	string	true	"container id"
// @Param			stdout		query	bool	false	"show stdout"
// @Param			stderr		query	bool	false	"show stderr"
// @Param			from		query	string	false	"from timestamp"
// @Param			to			query	string	false	"to timestamp"
// @Param			levels		query	string	false	"levels"
// @Success		200
// @Failure		400	{object}	apitypes.ErrorResponse
// @Failure		403	{object}	apitypes.ErrorResponse
// @Failure		404	{object}	apitypes.ErrorResponse
// @Failure		500	{object}	apitypes.ErrorResponse
// @Router			/docker/logs/{id} [get]
func Logs(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, apitypes.Error("container id is required"))
		return
	}

	var queryParams LogsQueryParams
	if err := c.ShouldBindQuery(&queryParams); err != nil {
		c.JSON(http.StatusBadRequest, apitypes.Error("invalid query params"))
		return
	}

	// TODO: implement levels

	dockerHost, ok := docker.GetDockerHostByContainerID(id)
	if !ok {
		c.JSON(http.StatusNotFound, apitypes.Error("container not found"))
		return
	}

	dockerClient, err := docker.NewClient(dockerHost)
	if err != nil {
		c.Error(apitypes.InternalServerError(err, "failed to create docker client"))
		return
	}

	defer dockerClient.Close()

	opts := container.LogsOptions{
		ShowStdout: queryParams.Stdout,
		ShowStderr: queryParams.Stderr,
		Since:      queryParams.Since,
		Until:      queryParams.Until,
		Timestamps: true,
		Follow:     true,
		Tail:       "100",
	}
	if queryParams.Levels != "" {
		opts.Details = true
	}

	logs, err := dockerClient.ContainerLogs(c.Request.Context(), id, opts)
	if err != nil {
		c.Error(apitypes.InternalServerError(err, "failed to get container logs"))
		return
	}
	defer logs.Close()

	manager, err := websocket.NewManagerWithUpgrade(c)
	if err != nil {
		c.Error(apitypes.InternalServerError(err, "failed to create websocket manager"))
		return
	}
	defer manager.Close()

	writer := manager.NewWriter(websocket.TextMessage)

	_, err = stdcopy.StdCopy(writer, writer, logs) // de-multiplex logs
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, task.ErrProgramExiting) {
			return
		}
		log.Err(err).
			Str("server", dockerHost).
			Str("container", id).
			Msg("failed to de-multiplex logs")
	}
}
