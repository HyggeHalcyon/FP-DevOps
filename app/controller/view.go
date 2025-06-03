package controller

import (
	"FP-DevOps/config"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type (
	ViewController interface {
		Index(ctx *gin.Context)
		Login(ctx *gin.Context)
		Register(ctx *gin.Context)
		Dashboard(ctx *gin.Context)
	}

	viewController struct {
		jwtService config.JWTService
	}
)

func NewViewController(jwt config.JWTService) ViewController {
	return &viewController{
		jwtService: jwt,
	}
}

func (c *viewController) Index(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "index.tmpl", gin.H{
		"title": "Welcome to Cloud File Manager",
		"env":   os.Getenv("ENV"),
	})
}

func (c *viewController) Login(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "login.tmpl", gin.H{
		"title": "This is the Login Page",
		"env":   os.Getenv("ENV"),
	})
}

func (c *viewController) Register(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "register.tmpl", gin.H{
		"title": "This is the Register Page",
		"env":   os.Getenv("ENV"),
	})
}

func (c *viewController) Dashboard(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "dashboard.tmpl", gin.H{
		"title": "This is the Dashboard Page",
		"env":   os.Getenv("ENV"),
	})
}
