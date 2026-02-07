package handler

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/leo-sagawa/homedocmanager/internal/config"
)

// AdminAuthMiddleware enforces token-based auth for admin endpoints.
// Mode: required | optional | disabled
func AdminAuthMiddleware() gin.HandlerFunc {
	return AdminAuthMiddlewareWith(
		strings.ToLower(strings.TrimSpace(config.AdminAuthMode)),
		strings.TrimSpace(config.AdminToken),
	)
}

// AdminAuthMiddlewareWith is a testable variant of AdminAuthMiddleware.
func AdminAuthMiddlewareWith(mode string, expectedToken string) gin.HandlerFunc {
	mode = strings.ToLower(strings.TrimSpace(mode))

	return func(c *gin.Context) {
		if mode == "disabled" {
			c.Next()
			return
		}

		expected := strings.TrimSpace(expectedToken)
		if expected == "" {
			log.Printf("Admin auth required but ADMIN_TOKEN is empty")
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "admin auth not configured"})
			return
		}

		provided := extractAdminToken(c)
		if provided == "" {
			if mode == "optional" {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing admin token"})
			return
		}

		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid admin token"})
			return
		}

		c.Next()
	}
}

func extractAdminToken(c *gin.Context) string {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return strings.TrimSpace(c.GetHeader("X-Admin-Token"))
}
