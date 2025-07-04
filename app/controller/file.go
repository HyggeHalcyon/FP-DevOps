package controller

import (
	"FP-DevOps/config"
	"FP-DevOps/constants"
	"FP-DevOps/dto"
	"FP-DevOps/service"
	"FP-DevOps/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

type (
	FileController interface {
		Create(ctx *gin.Context)
		UpdateByID(ctx *gin.Context)
		DeleteByID(ctx *gin.Context)
		GetFileByID(ctx *gin.Context)
		GetPaginated(ctx *gin.Context)
	}

	fileController struct {
		jwtService  config.JWTService
		fileService service.FileService
	}
)

func NewFileController(fs service.FileService, jwt config.JWTService) FileController {
	return &fileController{
		jwtService:  jwt,
		fileService: fs,
	}
}

func (c *fileController) Create(ctx *gin.Context) {
	var req dto.CreateFileRequest
	if err := ctx.ShouldBind(&req); err != nil {
		response := utils.BuildResponseFailed(dto.MESSAGE_FAILED_GET_DATA_FROM_BODY, err.Error(), nil)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, response)
		return
	}

	res, err := c.fileService.Create(ctx.Request.Context(), ctx.GetString(constants.CTX_KEY_USER_ID), req)
	if err != nil {
		response := utils.BuildResponseFailed(dto.MESSAGE_FAILED_CREATE_FILE, err.Error(), nil)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, response)
		return
	}

	response := utils.BuildResponseSuccess(dto.MESSAGE_SUCCESS_CREATE_FILE, res)
	ctx.JSON(http.StatusCreated, response)
}

func (c *fileController) UpdateByID(ctx *gin.Context) {
	var req dto.FileUpdate
	id := ctx.Param("id")

	if err := ctx.ShouldBind(&req); err != nil {
		response := utils.BuildResponseFailed(dto.MESSAGE_FAILED_GET_DATA_FROM_BODY, err.Error(), nil)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, response)
		return
	}

	res, err := c.fileService.Update(ctx.Request.Context(), ctx.GetString(constants.CTX_KEY_USER_ID), id, req)
	if err != nil {
		response := utils.BuildResponseFailed(dto.MESSAGE_FAILED_UPDATE_FILE, err.Error(), nil)
		if err == dto.ErrUnauthorizedFileAccess {
			ctx.AbortWithStatusJSON(http.StatusForbidden, response)
			return
		} else if err == dto.ErrFileNotFound {
			ctx.AbortWithStatusJSON(http.StatusNotFound, response)
			return
		} else {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, response)
		}
		return
	}

	response := utils.BuildResponseSuccess(dto.MESSAGE_SUCCESS_UPDATE_FILE, res)
	ctx.JSON(http.StatusOK, response)
}

func (c *fileController) DeleteByID(ctx *gin.Context) {
	id := ctx.Param("id")

	if err := c.fileService.Delete(ctx.Request.Context(), ctx.GetString(constants.CTX_KEY_USER_ID), id); err != nil {
		response := utils.BuildResponseFailed(dto.MESSAGE_FAILED_DELETE_FILE, err.Error(), nil)
		if err == dto.ErrUnauthorizedFileAccess {
			ctx.AbortWithStatusJSON(http.StatusForbidden, response)
		} else if err == dto.ErrFileNotFound {
			ctx.AbortWithStatusJSON(http.StatusNotFound, response)
		} else {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, response)
		}
		return
	}

	response := utils.BuildResponseSuccess(dto.MESSAGE_SUCCESS_DELETE_FILE, nil)
	ctx.JSON(http.StatusOK, response)
}

func (c *fileController) GetFileByID(ctx *gin.Context) {
	id := ctx.Param("id")
	view := ctx.Query("view")
	userID := ctx.GetString(constants.CTX_KEY_USER_ID)

	res, err := c.fileService.GetFile(ctx.Request.Context(), userID, id)
	if err != nil {
		if err == dto.ErrUnauthorizedFileAccess {
			ctx.HTML(http.StatusBadRequest, "privateError.tmpl", gin.H{
				"title":   "Unauthorized Access",
				"message": "You do not have permission to access this file.",
			})
			return
		}
		response := utils.BuildResponseFailed(dto.MESSAGE_FAILED_GET_FILE, err.Error(), nil)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, response)
		return
	}

	if view != "" {
		ctx.Header("Content-Disposition", "inline; filename="+res.Filename)
		ctx.Header("Content-Type", res.MimeType)
	} else {
		ctx.Header("Content-Disposition", "attachment; filename="+res.Filename)
		ctx.Header("Content-Type", "application/octet-stream")
	}

	ctx.Writer.Write(res.Content)
}

func (c *fileController) GetPaginated(ctx *gin.Context) {
	var req dto.PaginationQuery
	if err := ctx.ShouldBind(&req); err != nil {
		res := utils.BuildResponseFailed(dto.MESSAGE_FAILED_GET_DATA_FROM_BODY, err.Error(), nil)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, res)
		return
	}

	result, err := c.fileService.GetPaginated(ctx.Request.Context(), ctx.GetString(constants.CTX_KEY_USER_ID), req)
	if err != nil {
		response := utils.BuildResponseFailed(dto.MESSAGE_FAILED_GET_FILE, err.Error(), nil)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, response)
		return
	}

	res := utils.Response{
		Status:  true,
		Message: dto.MESSAGE_SUCCESS_GET_FILE,
		Data:    result.Data,
		Meta:    result.PaginationMetadata,
	}
	ctx.JSON(http.StatusOK, res)
}
