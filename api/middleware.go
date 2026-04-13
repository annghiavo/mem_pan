package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func authMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userIDHeader := ctx.GetHeader("x-user-id")
		if userIDHeader == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse(errors.New("missing x-user-id header")))
			return
		}

		userID, err := uuid.Parse(userIDHeader)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse(errors.New("invalid x-user-id")))
			return
		}

		ctx.Set(authorizationPayloadKey, &AuthPayload{UserID: userID})
		ctx.Next()
	}
}
