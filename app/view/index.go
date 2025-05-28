package view

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type (
	IndexView interface {
		Index(ctx *gin.Context)
	}

	welcomeView struct {
	}
)

func NewIndexView() IndexView {
	return &welcomeView{}
}

func (w *welcomeView) Index(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "index.tmpl", gin.H{
		"title": "Welcome to FP-DevOps",
		"env":   os.Getenv("ENV"),
	})
}
