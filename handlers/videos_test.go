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
	}
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
