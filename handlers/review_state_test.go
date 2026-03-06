package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestReviewState_GetEmpty(t *testing.T) {
	// Use a temp file for test
	tmp := t.TempDir() + "/test_review_state.json"
	os.Setenv("REVIEW_STATE_PATH", tmp)
	defer os.Unsetenv("REVIEW_STATE_PATH")

	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/review/state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var state ReviewState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(state.ReviewedIDs) != 0 {
		t.Errorf("expected empty reviewed_ids, got %v", state.ReviewedIDs)
	}
}

func TestReviewState_AddAndGet(t *testing.T) {
	tmp := t.TempDir() + "/test_review_state.json"
	os.Setenv("REVIEW_STATE_PATH", tmp)
	defer os.Unsetenv("REVIEW_STATE_PATH")

	r := setupRouter()

	// Add first ID
	req, _ := http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader(`{"post_id": 123}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var state ReviewState
	json.Unmarshal(w.Body.Bytes(), &state)
	if len(state.ReviewedIDs) != 1 || state.ReviewedIDs[0] != 123 {
		t.Errorf("expected [123], got %v", state.ReviewedIDs)
	}

	// Add second ID
	req, _ = http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader(`{"post_id": 456}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &state)
	if len(state.ReviewedIDs) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(state.ReviewedIDs))
	}

	// Add duplicate ID (should not add again)
	req, _ = http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader(`{"post_id": 123}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &state)
	if len(state.ReviewedIDs) != 2 {
		t.Errorf("expected 2 IDs after duplicate, got %d", len(state.ReviewedIDs))
	}

	// Verify the file was actually written
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}
	var fileState ReviewState
	json.Unmarshal(data, &fileState)
	if len(fileState.ReviewedIDs) != 2 {
		t.Errorf("expected 2 IDs in file, got %d", len(fileState.ReviewedIDs))
	}

	// GET should return the same state
	req, _ = http.NewRequest(http.MethodGet, "/api/review/state", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &state)
	if len(state.ReviewedIDs) != 2 {
		t.Errorf("expected 2 IDs on GET, got %d", len(state.ReviewedIDs))
	}
}

func TestReviewState_Clear(t *testing.T) {
	tmp := t.TempDir() + "/test_review_state.json"
	os.Setenv("REVIEW_STATE_PATH", tmp)
	defer os.Unsetenv("REVIEW_STATE_PATH")

	r := setupRouter()

	// Add some IDs first
	req, _ := http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader(`{"post_id": 111}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	req, _ = http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader(`{"post_id": 222}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Clear
	req, _ = http.NewRequest(http.MethodDelete, "/api/review/state", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var state ReviewState
	json.Unmarshal(w.Body.Bytes(), &state)
	if len(state.ReviewedIDs) != 0 {
		t.Errorf("expected empty after clear, got %v", state.ReviewedIDs)
	}
}

func TestReviewState_InvalidBody(t *testing.T) {
	tmp := t.TempDir() + "/test_review_state.json"
	os.Setenv("REVIEW_STATE_PATH", tmp)
	defer os.Unsetenv("REVIEW_STATE_PATH")

	r := setupRouter()

	req, _ := http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
