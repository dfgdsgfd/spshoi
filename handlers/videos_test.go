package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	{
		api.GET("/videos", GetVideos)
		api.POST("/videos/batch-toggle", BatchToggleVideos)
		api.POST("/videos/batch-disable", BatchDisableVideos)
	}
	r.GET("/review", ReviewPage)
	r.GET("/review.html", ReviewPage)
	return r
}

func TestGetVideos_DefaultParams(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/videos", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// The upstream might not be reachable in test, but we verify the endpoint exists
	if w.Code == http.StatusNotFound {
		t.Error("expected endpoint to exist, got 404")
	}
}

func TestGetVideos_WithParams(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/videos?page=2&per_page=10&search=test&order=ASC", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Error("expected endpoint to exist, got 404")
	}
}

func TestGetVideos_InvalidPage(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/videos?page=-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Error("expected endpoint to exist, got 404")
	}
}

func TestBatchToggleVideos_InvalidBody(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodPost, "/api/videos/batch-toggle", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBatchToggleVideos_EmptyVideos(t *testing.T) {
	r := setupRouter()

	body := BatchToggleRequest{Videos: []VideoToggleRequest{}}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, "/api/videos/batch-toggle", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty videos, got %d", w.Code)
	}
}

func TestBatchToggleVideos_ValidRequest(t *testing.T) {
	r := setupRouter()

	body := BatchToggleRequest{
		Videos: []VideoToggleRequest{
			{PostID: 1, Enable: false},
			{PostID: 2, Enable: true},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, "/api/videos/batch-toggle", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Endpoint should respond (may fail upstream, but should return 200 with results)
	if w.Code == http.StatusNotFound {
		t.Error("expected endpoint to exist, got 404")
	}

	if w.Code == http.StatusOK {
		var resp BatchToggleResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Errorf("failed to parse response: %v", err)
		}
		if resp.Total != 2 {
			t.Errorf("expected total 2, got %d", resp.Total)
		}
	}
}

func TestBatchDisableVideos_InvalidBody(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodPost, "/api/videos/batch-disable", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBatchDisableVideos_EmptyPostIDs(t *testing.T) {
	r := setupRouter()

	body := BatchDisableRequest{PostIDs: []int{}}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, "/api/videos/batch-disable", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty post_ids, got %d", w.Code)
	}
}

func TestBatchDisableVideos_ValidRequest(t *testing.T) {
	r := setupRouter()

	body := BatchDisableRequest{PostIDs: []int{1, 2, 3}}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, "/api/videos/batch-disable", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Endpoint should respond (may fail upstream, but should not return 404)
	if w.Code == http.StatusNotFound {
		t.Error("expected endpoint to exist, got 404")
	}
}

func TestGetVideos_WithSearchParam(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/videos?search=test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Error("expected endpoint to exist, got 404")
	}
}

func TestGetVideos_EmptySearch(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/videos?search=", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Error("expected endpoint to exist, got 404")
	}
}

func TestReviewPage(t *testing.T) {
	r := setupRouter()

	// Test both /review and /review.html paths serve the same page
	for _, path := range []string{"/review", "/review.html"} {
		t.Run(path, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "text/html; charset=utf-8" {
				t.Errorf("expected text/html content type, got %s", contentType)
			}

			body := w.Body.String()
			if len(body) == 0 {
				t.Error("expected non-empty body")
			}

			// Verify key HTML elements are present
			if !bytes.Contains(w.Body.Bytes(), []byte("视频审核")) {
				t.Error("expected review page to contain '视频审核'")
			}
			if !bytes.Contains(w.Body.Bytes(), []byte("hls.js")) {
				t.Error("expected review page to contain hls.js reference")
			}
			if !bytes.Contains(w.Body.Bytes(), []byte("btnApprove")) {
				t.Error("expected review page to contain approve button")
			}
			if !bytes.Contains(w.Body.Bytes(), []byte("btnReject")) {
				t.Error("expected review page to contain reject button")
			}
		})
	}
}
