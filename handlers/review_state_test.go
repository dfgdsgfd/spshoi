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
	req, _ := http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader(`{"post_id": 123, "status": "approved"}`))
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
	req, _ = http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader(`{"post_id": 456, "status": "rejected"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &state)
	if len(state.ReviewedIDs) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(state.ReviewedIDs))
	}

	// Add duplicate ID (should not add again)
	req, _ = http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader(`{"post_id": 123, "status": "approved"}`))
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
	req, _ := http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader(`{"post_id": 111, "status": "approved"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	req, _ = http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader(`{"post_id": 222, "status": "rejected"}`))
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

func TestReviewState_Statuses(t *testing.T) {
	tmp := t.TempDir() + "/test_review_state.json"
	os.Setenv("REVIEW_STATE_PATH", tmp)
	defer os.Unsetenv("REVIEW_STATE_PATH")

	r := setupRouter()

	for _, body := range []string{
		`{"post_id": 123, "status": "approved"}`,
		`{"post_id": 456, "status": "rejected"}`,
	} {
		req, _ := http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", body, w.Code)
		}
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/review/state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var state ReviewState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if state.Statuses[123] != reviewStatusApproved || state.Statuses[456] != reviewStatusRejected {
		t.Errorf("unexpected statuses: %v", state.Statuses)
	}
	if len(state.ReviewedIDs) != 2 {
		t.Errorf("expected two completed reviews, got %v", state.ReviewedIDs)
	}

	req, _ = http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader(`{"post_id": 1, "status": "unknown"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid status, got %d", w.Code)
	}
}

func TestReviewState_LegacyCompletedRecordRequiresRecheck(t *testing.T) {
	tmp := t.TempDir() + "/test_review_state.json"
	os.Setenv("REVIEW_STATE_PATH", tmp)
	defer os.Unsetenv("REVIEW_STATE_PATH")

	if err := os.WriteFile(tmp, []byte(`{"reviewed_ids":[123]}`), 0600); err != nil {
		t.Fatalf("failed to write legacy state: %v", err)
	}

	r := setupRouter()
	req, _ := http.NewRequest(http.MethodGet, "/api/review/state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var state ReviewState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, ok := state.Statuses[123]; ok {
		t.Errorf("legacy record should not have a completed result, got %q", state.Statuses[123])
	}
	if len(state.ReviewedIDs) != 0 {
		t.Errorf("legacy record without a result should not be completed, got %v", state.ReviewedIDs)
	}
}

func TestReviewState_RecheckAllKeepsResults(t *testing.T) {
	tmp := t.TempDir() + "/test_review_state.json"
	os.Setenv("REVIEW_STATE_PATH", tmp)
	defer os.Unsetenv("REVIEW_STATE_PATH")

	r := setupRouter()
	for _, body := range []string{
		`{"post_id": 123, "status": "approved"}`,
		`{"recheck_all": true}`,
	} {
		req, _ := http.NewRequest(http.MethodPost, "/api/review/state", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", body, w.Code)
		}
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/review/state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var state ReviewState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !state.RecheckAll || state.Statuses[123] != reviewStatusApproved || len(state.ReviewedIDs) != 1 {
		t.Errorf("recheck mode should preserve results, got %#v", state)
	}
}
