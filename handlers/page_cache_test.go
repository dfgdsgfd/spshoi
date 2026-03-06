package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestPageCache_GetEmpty(t *testing.T) {
	tmp := t.TempDir() + "/test_page_cache.json"
	os.Setenv("PAGE_CACHE_PATH", tmp)
	defer os.Unsetenv("PAGE_CACHE_PATH")

	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/review/pages", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var state PageCacheState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(state.Pages) != 0 {
		t.Errorf("expected empty pages, got %v", state.Pages)
	}
}

func TestPageCache_SaveAndGet(t *testing.T) {
	tmp := t.TempDir() + "/test_page_cache.json"
	os.Setenv("PAGE_CACHE_PATH", tmp)
	defer os.Unsetenv("PAGE_CACHE_PATH")

	r := setupRouter()

	// Save page 1
	body := `{"page": "1", "data": {"posts": [{"id": 100, "title": "Test"}], "total_pages": 3}}`
	req, _ := http.NewRequest(http.MethodPost, "/api/review/pages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	// Save page 2
	body2 := `{"page": "2", "data": {"posts": [{"id": 200, "title": "Test2"}], "total_pages": 3}}`
	req, _ = http.NewRequest(http.MethodPost, "/api/review/pages", strings.NewReader(body2))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// GET should return both pages
	req, _ = http.NewRequest(http.MethodGet, "/api/review/pages", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var state PageCacheState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(state.Pages) != 2 {
		t.Errorf("expected 2 pages, got %d", len(state.Pages))
	}
	if _, ok := state.Pages["1"]; !ok {
		t.Error("expected page 1 to be cached")
	}
	if _, ok := state.Pages["2"]; !ok {
		t.Error("expected page 2 to be cached")
	}

	// Verify file was written
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("failed to read cache file: %v", err)
	}
	var fileState PageCacheState
	if err := json.Unmarshal(data, &fileState); err != nil {
		t.Fatalf("failed to parse cache file: %v", err)
	}
	if len(fileState.Pages) != 2 {
		t.Errorf("expected 2 pages in file, got %d", len(fileState.Pages))
	}
}

func TestPageCache_Clear(t *testing.T) {
	tmp := t.TempDir() + "/test_page_cache.json"
	os.Setenv("PAGE_CACHE_PATH", tmp)
	defer os.Unsetenv("PAGE_CACHE_PATH")

	r := setupRouter()

	// Save a page first
	body := `{"page": "1", "data": {"posts": [{"id": 100}]}}`
	req, _ := http.NewRequest(http.MethodPost, "/api/review/pages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Clear
	req, _ = http.NewRequest(http.MethodDelete, "/api/review/pages", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var state PageCacheState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(state.Pages) != 0 {
		t.Errorf("expected empty after clear, got %v", state.Pages)
	}
}

func TestPageCache_InvalidBody(t *testing.T) {
	tmp := t.TempDir() + "/test_page_cache.json"
	os.Setenv("PAGE_CACHE_PATH", tmp)
	defer os.Unsetenv("PAGE_CACHE_PATH")

	r := setupRouter()

	req, _ := http.NewRequest(http.MethodPost, "/api/review/pages", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPageCache_OverwritePage(t *testing.T) {
	tmp := t.TempDir() + "/test_page_cache.json"
	os.Setenv("PAGE_CACHE_PATH", tmp)
	defer os.Unsetenv("PAGE_CACHE_PATH")

	r := setupRouter()

	// Save page 1
	body := `{"page": "1", "data": {"posts": [{"id": 100}]}}`
	req, _ := http.NewRequest(http.MethodPost, "/api/review/pages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Overwrite page 1 with new data
	body2 := `{"page": "1", "data": {"posts": [{"id": 999}]}}`
	req, _ = http.NewRequest(http.MethodPost, "/api/review/pages", strings.NewReader(body2))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// GET should have the updated data
	req, _ = http.NewRequest(http.MethodGet, "/api/review/pages", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var state PageCacheState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(state.Pages) != 1 {
		t.Errorf("expected 1 page, got %d", len(state.Pages))
	}

	var pageData map[string]interface{}
	if err := json.Unmarshal(state.Pages["1"], &pageData); err != nil {
		t.Fatalf("failed to parse page data: %v", err)
	}
	posts := pageData["posts"].([]interface{})
	post := posts[0].(map[string]interface{})
	if int(post["id"].(float64)) != 999 {
		t.Errorf("expected overwritten id 999, got %v", post["id"])
	}
}
