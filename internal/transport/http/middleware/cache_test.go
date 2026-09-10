package middleware

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// serveWithNoCache прогоняет запрос через middleware и отдаёт ответ.
func serveWithNoCache(t *testing.T, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()

	router := gin.New()
	router.GET("/probe", NoCache(), handler)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/probe", nil))
	return recorder
}

func TestNoCacheForbidsStoring(t *testing.T) {
	recorder := serveWithNoCache(t, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	control := recorder.Header().Get("Cache-Control")
	for _, directive := range []string{"no-store", "no-cache", "must-revalidate", "private"} {
		if !strings.Contains(control, directive) {
			t.Fatalf("Cache-Control = %q, want it to contain %q", control, directive)
		}
	}

	if got := recorder.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("Pragma = %q, want no-cache", got)
	}
}

// Ответ зависит от того, чей токен пришёл. Без Vary общий кэш на пути
// вправе отдать одному пользователю то, что положил туда другой, —
// то есть показать чужие задачи.
func TestNoCacheVariesByIdentity(t *testing.T) {
	recorder := serveWithNoCache(t, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	vary := recorder.Header().Values("Vary")
	for _, header := range []string{"Authorization", "X-User-ID"} {
		if !containsValue(vary, header) {
			t.Fatalf("Vary = %v, want it to list %q", vary, header)
		}
	}
}

// Заголовки должны стоять и на отказах: закэшированный 401 продолжал бы
// выкидывать пользователя из системы уже после успешного входа.
func TestNoCacheAppliesToErrorResponses(t *testing.T) {
	recorder := serveWithNoCache(t, func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	})

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	if control := recorder.Header().Get("Cache-Control"); !strings.Contains(control, "no-store") {
		t.Fatalf("Cache-Control = %q, want no-store on an error response", control)
	}
}

// Vary добавляется, а не перезаписывается: CORS-слой шлюза уже кладёт
// туда Origin, и затирать его нельзя.
func TestNoCacheKeepsExistingVary(t *testing.T) {
	router := gin.New()
	router.GET("/probe", func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Origin")
		c.Next()
	}, NoCache(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/probe", nil))

	vary := recorder.Header().Values("Vary")
	if !containsValue(vary, "Origin") {
		t.Fatalf("Vary = %v, want it to keep Origin", vary)
	}
	if !containsValue(vary, "Authorization") {
		t.Fatalf("Vary = %v, want it to add Authorization", vary)
	}
}

func containsValue(values []string, want string) bool {
	return slices.Contains(values, want)
}
