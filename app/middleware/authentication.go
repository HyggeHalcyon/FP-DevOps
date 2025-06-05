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
		var userID, userRole string

		authHeader := ctx.GetHeader("Authorization")
		cookieToken, err := ctx.Cookie("jwt")
		if err != nil {
			cookieToken = ""
		}

		if authHeader != "" {
			if !strings.Contains(authHeader, "Bearer ") {
				ctx.Abort()
				return
			}

			authHeader = strings.Replace(authHeader, "Bearer ", "", -1)
			userID, userRole, err = jwtService.GetPayloadInsideToken(authHeader)
			if err != nil {
				if err.Error() == dto.ErrTokenExpired.Error() {
					ctx.Abort()
					return
				}
				ctx.Abort()
				return
			}
		} else if cookieToken != "" {
			cookieToken, err := ctx.Cookie("jwt")
			if err != nil || strings.TrimSpace(cookieToken) == "" {
				ctx.Abort()
				return
			}

			userID, userRole, err = jwtService.GetPayloadInsideToken(cookieToken)
			if err != nil {
				ctx.SetCookie("jwt", "", -1, "/", "", false, true)
				ctx.Abort()
				return
			}
		}

		ctx.Set(constants.CTX_KEY_TOKEN, authHeader)
		ctx.Set(constants.CTX_KEY_USER_ID, userID)
		ctx.Set(constants.CTX_KEY_ROLE_NAME, userRole)
		ctx.Next()
	}
}

func ForceLogin(jwtService config.JWTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		cookieToken, err := ctx.Cookie("jwt")
		if err != nil || strings.TrimSpace(cookieToken) == "" {
			ctx.Redirect(http.StatusFound, "/login")
			ctx.Abort()
			return
		}

		userID, userRole, err := jwtService.GetPayloadInsideToken(cookieToken)
		if err != nil {
			ctx.SetCookie("jwt", "", -1, "/", "", false, true)
			ctx.Redirect(http.StatusFound, "/login")
			ctx.Abort()
			return
		}

		ctx.Set(constants.CTX_KEY_TOKEN, cookieToken)
		ctx.Set(constants.CTX_KEY_USER_ID, userID)
		ctx.Set(constants.CTX_KEY_ROLE_NAME, userRole)
		ctx.Next()
	}
}

func abortTokenInvalid(ctx *gin.Context) {
	response := utils.BuildResponseFailed(dto.MESSAGE_FAILED_VERIFY_TOKEN, dto.ErrTokenInvalid.Error(), nil)
	ctx.AbortWithStatusJSON(http.StatusUnauthorized, response)
}
