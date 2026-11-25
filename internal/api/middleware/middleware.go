package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger returns structured JSON request logging middleware.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

// Recovery catches panics and returns 500.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "err", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": map[string]any{
						"code":    "INTERNAL_ERROR",
						"message": "internal server error",
					},
				})
			}
		}()
		c.Next()
	}
}

// OwnerID is a stub for Week 1.
// Week 4 replaces this with real JWT parsing.
const StubOwnerID = "user_dev"

// Auth is a stub middleware for Week 1 that sets a fixed owner_id.
// Replace with JWT validation in Week 4.
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO Week 4: parse Authorization: Bearer <jwt>
		c.Set("owner_id", StubOwnerID)
		c.Next()
	}
}

// GetOwnerID reads the owner_id set by Auth middleware.
func GetOwnerID(c *gin.Context) string {
	v, _ := c.Get("owner_id")
	s, _ := v.(string)
	if s == "" {
		return StubOwnerID
	}
	return s
}
