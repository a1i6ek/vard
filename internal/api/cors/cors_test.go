package cors

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const panelOrigin = "http://localhost:5173"

// corsEngine is a router with nothing on it but CORS and one route to reach.
func corsEngine() *gin.Engine {
	r := gin.New()
	r.Use(New())
	r.GET("/thing", func(c *gin.Context) { c.Status(http.StatusOK) })

	return r
}

// send makes a request with the given origin, or none when it is empty.
func send(r *gin.Engine, method, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/thing", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	return w
}

// Every app is served from its API's origin, so no origin may call the
// cookie-authenticated API from a browser: the panel's own origin, one that
// merely looks like it, and a stranger are all told nothing, so no other site
// can act as whoever is signed in. A preflight is still answered, and allows
// nothing.
func TestCORSTellsEveryOriginNothingOutsideTheProviderEndpoints(t *testing.T) {
	origins := []string{
		panelOrigin,
		"http://localhost:5174",
		"https://localhost:5173",
		"http://localhost:5173.evil.test",
		"http://evil.test",
		"",
	}

	for _, origin := range origins {
		for _, method := range []string{http.MethodGet, http.MethodOptions} {
			t.Run(method+" "+origin, func(t *testing.T) {
				w := send(corsEngine(), method, origin)

				want := http.StatusOK
				if method == http.MethodOptions {
					want = http.StatusNoContent
				}
				if w.Code != want {
					t.Errorf("status = %d, want %d", w.Code, want)
				}
				for _, header := range []string{
					"Access-Control-Allow-Origin",
					"Access-Control-Allow-Credentials",
					"Access-Control-Allow-Methods",
					"Access-Control-Allow-Headers",
				} {
					if got := w.Header().Get(header); got != "" {
						t.Errorf("%s = %q, want it unset", header, got)
					}
				}
			})
		}
	}
}

// A preflight is answered by the middleware and never reaches the route.
func TestCORSAnswersThePreflightItself(t *testing.T) {
	reached := false

	r := gin.New()
	r.Use(New())
	r.OPTIONS("/thing", func(c *gin.Context) { reached = true })

	send(r, http.MethodOptions, panelOrigin)

	if reached {
		t.Error("the preflight reached the route, want it answered by the middleware")
	}
}

// The provider endpoints a single-page app calls are open to any origin, and
// never with credentials; the admin API stays closed to the same origin.
func TestCORSPublicProviderEndpoints(t *testing.T) {
	r := gin.New()
	r.Use(New())
	r.POST("/oauth2/token", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/.well-known/jwks.json", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/oauth2/authorize", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/admin/me", func(c *gin.Context) { c.Status(http.StatusOK) })

	call := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Origin", "https://spa.example.com")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	for _, path := range []string{"/oauth2/token", "/.well-known/jwks.json"} {
		method := http.MethodGet
		if path == "/oauth2/token" {
			method = http.MethodPost
		}

		w := call(method, path)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("%s: Allow-Origin = %q, want *", path, got)
		}
		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Errorf("%s: Allow-Credentials = %q, want none", path, got)
		}
	}

	if w := call(http.MethodOptions, "/oauth2/token"); w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("token preflight = %d, %q", w.Code, w.Header().Get("Access-Control-Allow-Origin"))
	}

	for _, path := range []string{"/oauth2/authorize", "/api/v1/admin/me"} {
		if got := call(http.MethodGet, path).Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("%s: Allow-Origin = %q, want none", path, got)
		}
	}
}

// A browser app using DPoP sends its proof in a DPoP header and has to read
// the server's DPoP-Nonce and WWW-Authenticate back; a header the preflight
// does not allow is refused by the browser, and one not exposed is hidden
// from script. The pushed request endpoint is one such app calls too.
func TestCORSLetsABrowserAppUseDPoP(t *testing.T) {
	r := gin.New()
	r.Use(New())

	for _, path := range []string{"/oauth2/token", "/oauth2/par", "/oauth2/userinfo", "/oauth2/revoke"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, path, nil)
			req.Header.Set("Origin", "https://spa.example.com")
			req.Header.Set("Access-Control-Request-Headers", "dpop, content-type")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
				t.Errorf("Allow-Origin = %q, want *", got)
			}
			if got := w.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "DPoP") {
				t.Errorf("Allow-Headers = %q, want DPoP among them", got)
			}
			for _, header := range []string{"DPoP-Nonce", "WWW-Authenticate"} {
				if got := w.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(got, header) {
					t.Errorf("Expose-Headers = %q, want %s among them", got, header)
				}
			}
		})
	}
}
