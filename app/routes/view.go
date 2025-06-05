package routes

import (
	"FP-DevOps/config"
	"FP-DevOps/controller"
	"FP-DevOps/middleware"

	"github.com/gin-gonic/gin"
)

func View(route *gin.Engine, view controller.ViewController, jwtService config.JWTService) {
	routes := route.Group("")
	{
		routes.GET("/", view.Index)
		routes.GET("/login", view.Login)
		routes.GET("/register", view.Register)
		routes.GET("/dashboard", middleware.ForceLogin(jwtService), view.Dashboard)
	}
}
