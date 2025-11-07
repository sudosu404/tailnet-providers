package routeApi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	statequery "github.com/sudosu404/providers/internal/config/query"
	"github.com/sudosu404/go-utils/http/httpheaders"
	"github.com/sudosu404/go-utils/http/websocket"

	_ "github.com/sudosu404/go-utils/apitypes"
)

// @x-id				"providers"
// @BasePath		/api/v1
// @Summary		List route providers
// @Description	List route providers
// @Tags			route,websocket
// @Accept			json
// @Produce		json
// @Success		200	{array}		statequery.RouteProviderListResponse
// @Failure		403	{object}	apitypes.ErrorResponse
// @Failure		500	{object}	apitypes.ErrorResponse
// @Router			/route/providers [get]
func Providers(c *gin.Context) {
	if httpheaders.IsWebsocket(c.Request.Header) {
		websocket.PeriodicWrite(c, 5*time.Second, func() (any, error) {
			return statequery.RouteProviderList(), nil
		})
	} else {
		c.JSON(http.StatusOK, statequery.RouteProviderList())
	}
}
