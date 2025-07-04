package routes

import (
	"FP-DevOps/config"
	"FP-DevOps/controller"
	"FP-DevOps/middleware"

	"github.com/gin-gonic/gin"
)

func File(route *gin.Engine, fileController controller.FileController, jwtService config.JWTService) {
	routes := route.Group("/api/file")
	{
		routes.GET("/:id", middleware.AuthenticateIfExists(jwtService), fileController.GetFileByID)
		routes.GET("", middleware.Authenticate(jwtService), fileController.GetPaginated)
		routes.POST("", middleware.Authenticate(jwtService), fileController.Create)
		routes.PATCH("/:id", middleware.Authenticate(jwtService), fileController.UpdateByID)
		routes.DELETE("/:id", middleware.Authenticate(jwtService), fileController.DeleteByID)
	}
}
