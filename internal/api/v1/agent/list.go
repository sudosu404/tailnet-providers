package agentapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sudosu404/providers/agent/pkg/agent"
	"github.com/sudosu404/go-utils/http/httpheaders"
	"github.com/sudosu404/go-utils/http/websocket"

	_ "github.com/sudosu404/go-utils/apitypes"
)

// @x-id				"list"
// @BasePath		/api/v1
// @Summary		List agents
// @Description	List agents
// @Tags			agent,websocket
// @Accept			json
// @Produce		json
// @Success		200	{array}		Agent
// @Failure		403	{object}	apitypes.ErrorResponse
// @Router			/agent/list [get]
func List(c *gin.Context) {
	if httpheaders.IsWebsocket(c.Request.Header) {
		websocket.PeriodicWrite(c, 10*time.Second, func() (any, error) {
			return agent.ListAgents(), nil
		})
	} else {
		c.JSON(http.StatusOK, agent.ListAgents())
	}
}
