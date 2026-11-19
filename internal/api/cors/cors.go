// Package cors opens the provider endpoints a browser app calls to every
// origin, without credentials, and the rest of the API to none: each app is
// served from its API's origin, so nothing else needs to call it.
package cors

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// A browser app sends its DPoP proof in a header of that name, and must
	// be able to read the provider's challenge and nonce back; a header not
	// exposed is hidden from script, which silently breaks DPoP.
	publicHeaders  = "Authorization, Content-Type, DPoP"
	exposedHeaders = "DPoP-Nonce, WWW-Authenticate"
)

// New opens the provider endpoints a browser app calls directly (token, PAR,
// userinfo, revocation, introspection, discovery, JWKS) to every origin,
// without credentials: they never read a cookie, and the client proves itself
// in the request. Every other path tells no origin anything, so no other site
// can act as the signed-in user.
func New() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if public(c.Request.URL.Path) {
			if origin != "" {
				h := c.Writer.Header()
				h.Set("Access-Control-Allow-Origin", "*")
				h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				h.Set("Access-Control-Allow-Headers", publicHeaders)
				h.Set("Access-Control-Expose-Headers", exposedHeaders)
			}

			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}

			c.Next()
			return
		}

		// Preflights are answered here for anyone; the browser enforces the
		// result, which allows nothing.
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// publicPaths are the provider endpoints any origin may call. Authorize and
// logout are excluded: browsers navigate there, and they read the session
// cookie.
var publicPaths = []string{
	"/oauth2/token",
	"/oauth2/par",
	"/oauth2/userinfo",
	"/oauth2/revoke",
	"/oauth2/introspect",
}

func public(path string) bool {
	return slices.Contains(publicPaths, path) || strings.HasPrefix(path, "/.well-known/")
}
