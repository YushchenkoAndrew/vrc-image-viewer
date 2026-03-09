package middleware

import (
	"github.com/gin-gonic/gin"
)

func (c *Middleware) Default() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()
	}
}
