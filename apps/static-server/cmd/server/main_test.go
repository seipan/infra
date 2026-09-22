package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/yamasaki/static-server/internal/handler"
)

// newTestRouter builds the real router around a handler with no storage behind
// it. Nothing here reaches the static handler, which is the point: the health
// endpoint must answer without a backend.
func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetOutput(nopWriter{})
	return newRouter(handler.NewStaticHandler(nil, nil, 0, logger), logger)
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestHealthzAnswersWithoutStorage(t *testing.T) {
	r := newTestRouter(t)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, healthPath, nil))

		if w.Code != http.StatusOK {
			t.Errorf("%s %s: status = %d, want %d", method, healthPath, w.Code, http.StatusOK)
		}
	}
}

func TestHealthzDoesNotCollideWithTheCatchAll(t *testing.T) {
	// gin cannot hold a root catch-all route and /healthz at once, so the
	// static handler is registered through NoRoute. Guard that arrangement:
	// building the router must not panic, and a normal path must still reach
	// the static handler rather than the health endpoint.
	r := newTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/some/page", nil)
	req.Host = "example.com" // two labels, so no bucket can be derived
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d (the static handler should have rejected the host)", w.Code, http.StatusBadRequest)
	}
}

func TestUnsupportedMethodsAreRejected(t *testing.T) {
	r := newTestRouter(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/some/page", nil))

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}
