package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestPageCache_GetDefault(t *testing.T) {
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
	if state.CurrentPage != 1 {
		t.Errorf("expected current_page 1, got %d", state.CurrentPage)
	}
}

func TestPageCache_SaveAndGet(t *testing.T) {
	tmp := t.TempDir() + "/test_page_cache.json"
	os.Setenv("PAGE_CACHE_PATH", tmp)
	defer os.Unsetenv("PAGE_CACHE_PATH")

	r := setupRouter()

	// Save page 5
	body := `{"current_page": 5}`
	req, _ := http.NewRequest(http.MethodPost, "/api/review/pages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var saved PageCacheState
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if saved.CurrentPage != 5 {
		t.Errorf("expected current_page 5, got %d", saved.CurrentPage)
	}

	// GET should return page 5
	req, _ = http.NewRequest(http.MethodGet, "/api/review/pages", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var state PageCacheState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if state.CurrentPage != 5 {
		t.Errorf("expected current_page 5, got %d", state.CurrentPage)
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
	if fileState.CurrentPage != 5 {
		t.Errorf("expected current_page 5 in file, got %d", fileState.CurrentPage)
	}
}

func TestPageCache_Clear(t *testing.T) {
	tmp := t.TempDir() + "/test_page_cache.json"
	os.Setenv("PAGE_CACHE_PATH", tmp)
	defer os.Unsetenv("PAGE_CACHE_PATH")

	r := setupRouter()

	// Save page 10 first
	body := `{"current_page": 10}`
	req, _ := http.NewRequest(http.MethodPost, "/api/review/pages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Clear (reset to page 1)
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
	if state.CurrentPage != 1 {
		t.Errorf("expected current_page 1 after clear, got %d", state.CurrentPage)
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

	// Save page 3
	body := `{"current_page": 3}`
	req, _ := http.NewRequest(http.MethodPost, "/api/review/pages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Overwrite with page 7
	body2 := `{"current_page": 7}`
	req, _ = http.NewRequest(http.MethodPost, "/api/review/pages", strings.NewReader(body2))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// GET should return page 7
	req, _ = http.NewRequest(http.MethodGet, "/api/review/pages", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var state PageCacheState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if state.CurrentPage != 7 {
		t.Errorf("expected current_page 7, got %d", state.CurrentPage)
	}
}
