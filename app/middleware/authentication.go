package middleware

import (
	"net/http"
	"strings"

	"FP-DevOps/config"
	"FP-DevOps/constants"
	"FP-DevOps/dto"
	"FP-DevOps/utils"

	"github.com/gin-gonic/gin"
)

func Authenticate(jwtService config.JWTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			response := utils.BuildResponseFailed(dto.MESSAGE_FAILED_VERIFY_TOKEN, dto.ErrTokenNotFound.Error(), nil)
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, response)
			return
		}

		if !strings.Contains(authHeader, "Bearer ") {
			abortTokenInvalid(ctx)
			return
		}

		authHeader = strings.Replace(authHeader, "Bearer ", "", -1)
		userID, userRole, err := jwtService.GetPayloadInsideToken(authHeader)
		if err != nil {
			if err.Error() == dto.ErrTokenExpired.Error() {
				response := utils.BuildResponseFailed(dto.MESSAGE_FAILED_VERIFY_TOKEN, dto.ErrTokenExpired.Error(), nil)
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, response)
				return
			}
			response := utils.BuildResponseFailed(dto.MESSAGE_FAILED_VERIFY_TOKEN, err.Error(), nil)
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, response)
			return
		}

		ctx.Set(constants.CTX_KEY_TOKEN, authHeader)
		ctx.Set(constants.CTX_KEY_USER_ID, userID)
		ctx.Set(constants.CTX_KEY_ROLE_NAME, userRole)
		ctx.Next()
	}
}

func AuthenticateIfExists(jwtService config.JWTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			return
		}

		if !strings.Contains(authHeader, "Bearer ") {
			abortTokenInvalid(ctx)
			return
		}

		authHeader = strings.Replace(authHeader, "Bearer ", "", -1)
		userID, userRole, err := jwtService.GetPayloadInsideToken(authHeader)
		if err != nil {
			if err.Error() == dto.ErrTokenExpired.Error() {
				return
			}
			return
		}

		ctx.Set(constants.CTX_KEY_TOKEN, authHeader)
		ctx.Set(constants.CTX_KEY_USER_ID, userID)
		ctx.Set(constants.CTX_KEY_ROLE_NAME, userRole)
		ctx.Next()
	}
}

func abortTokenInvalid(ctx *gin.Context) {
	response := utils.BuildResponseFailed(dto.MESSAGE_FAILED_VERIFY_TOKEN, dto.ErrTokenInvalid.Error(), nil)
	ctx.AbortWithStatusJSON(http.StatusUnauthorized, response)
}
