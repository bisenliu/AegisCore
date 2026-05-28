package router

import (
	"net/http"

	"github.com/aegiscore/user-services/internal/controller"
	"github.com/gin-gonic/gin"
)

type RouteParams struct {
	UserController *controller.UserController
}

func RegisterRoutes(engine *gin.Engine, params RouteParams) {
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "aegiscore-user-services"})
	})

	v1 := engine.Group("/api/v1")
	{
		users := v1.Group("/users")
		users.GET("/:id", params.UserController.GetByID)
	}
}
