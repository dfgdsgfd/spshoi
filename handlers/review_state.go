package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

const defaultReviewStatePath = "review_state.json"

const (
	reviewStatusApproved       = "approved"
	reviewStatusRejected       = "rejected"
	reviewStatusRecheck        = "recheck"
	legacyReviewStatusReviewed = "reviewed"
)

// ReviewState stores the review result for each video. ReviewedIDs is kept for
// compatibility with existing state files and clients; Statuses holds the
// detailed result used by the review UI. Completed reviews always have an
// approved or rejected result.
type ReviewState struct {
	ReviewedIDs []int          `json:"reviewed_ids"`
	Statuses    map[int]string `json:"statuses"`
	RecheckAll  bool           `json:"recheck_all"`
}

// ReviewStatusRequest updates one video's detailed review status.
type ReviewStatusRequest struct {
	PostID     int    `json:"post_id" example:"12345"`
	Status     string `json:"status" enums:"approved,rejected" example:"approved"`
	RecheckAll *bool  `json:"recheck_all"`
}

// BatchReviewItem describes one video's AI or bulk-review result.
type BatchReviewItem struct {
	PostID int    `json:"post_id" example:"12345"`
	Status string `json:"status" enums:"approved,rejected" example:"approved"`
}

// BatchReviewRequest supports either per-video statuses or one status for a list of IDs.
type BatchReviewRequest struct {
	Videos          []BatchReviewItem `json:"videos"`
	PostIDs         []int             `json:"post_ids" example:"1,2,3"`
	Status          string            `json:"status" enums:"approved,rejected" example:"approved"`
	DisableRejected *bool             `json:"disable_rejected" example:"true"`
}

type BatchReviewResult struct {
	PostID   int    `json:"post_id"`
	Status   string `json:"status"`
	Success  bool   `json:"success"`
	Disabled bool   `json:"disabled"`
	Message  string `json:"message"`
}

type BatchReviewResponse struct {
	Success  int                `json:"success"`
	Failed   int                `json:"failed"`
	Total    int                `json:"total"`
	Disabled int                `json:"disabled"`
	Results  []BatchReviewResult `json:"results"`
	State    *ReviewState       `json:"state"`
}

var reviewStateMu sync.Mutex

func getReviewStatePath() string {
	if v := os.Getenv("REVIEW_STATE_PATH"); v != "" {
		return v
	}
	return defaultReviewStatePath
}

func loadReviewState() (*ReviewState, error) {
	path := getReviewStatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newReviewState(), nil
		}
		return nil, err
	}
	var state ReviewState
	if err := json.Unmarshal(data, &state); err != nil {
		return newReviewState(), nil
	}
	normalizeReviewState(&state)
	return &state, nil
}

func newReviewState() *ReviewState {
	return &ReviewState{ReviewedIDs: []int{}, Statuses: map[int]string{}}
}

func normalizeReviewState(state *ReviewState) {
	if state.ReviewedIDs == nil {
		state.ReviewedIDs = []int{}
	}
	if state.Statuses == nil {
		state.Statuses = map[int]string{}
	}

	for id, status := range state.Statuses {
		if status == legacyReviewStatusReviewed || status == reviewStatusRecheck {
			// Neither legacy "reviewed" nor the former per-video recheck value
			// contains a pass/reject conclusion. Keep the video in recheck
			// without retaining a misleading completed result.
			delete(state.Statuses, id)
		}
	}

	// A recheck is deliberately not counted as completed. Rebuild the legacy
	// list from statuses so it always agrees with the displayed count.
	completed := make([]int, 0, len(state.Statuses))
	for id, status := range state.Statuses {
		if status == reviewStatusApproved || status == reviewStatusRejected {
			completed = append(completed, id)
		}
	}
	state.ReviewedIDs = completed
}

func isValidReviewStatus(status string) bool {
	switch status {
	case reviewStatusApproved, reviewStatusRejected:
		return true
	default:
		return false
	}
}

func saveReviewState(state *ReviewState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getReviewStatePath(), data, 0600)
}

// GetReviewState godoc
// @Summary Get review state
// @Description Returns the detailed review status and compatibility list of completed video IDs stored in server-side JSON file
// @Tags review
// @Produce json
// @Success 200 {object} ReviewState
// @Failure 500 {object} ErrorResponse
// @Router /review/state [get]
func GetReviewState(c *gin.Context) {
	reviewStateMu.Lock()
	defer reviewStateMu.Unlock()

	state, err := loadReviewState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to load review state: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, state)
}

// AddReviewedID godoc
// @Summary Add reviewed video ID
// @Description Save a video's approved or rejected result, or toggle recheck mode for all videos without clearing their results.
// @Tags review
// @Accept json
// @Produce json
// @Param request body ReviewStatusRequest true "Video review status"
// @Success 200 {object} ReviewState
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /review/state [post]
func AddReviewedID(c *gin.Context) {
	var req ReviewStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	reviewStateMu.Lock()
	defer reviewStateMu.Unlock()

	state, err := loadReviewState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to load review state: " + err.Error()})
		return
	}

	if req.RecheckAll != nil {
		state.RecheckAll = *req.RecheckAll
	} else {
		if req.PostID < 1 {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "post_id is required"})
			return
		}
		if !isValidReviewStatus(req.Status) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "approved or rejected status is required"})
			return
		}
		state.Statuses[req.PostID] = req.Status
	}

	normalizeReviewState(state)
	if err := saveReviewState(state); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to save review state: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, state)
}

// BatchReview godoc
// @Summary Batch save video review results
// @Description Save approved or rejected results for multiple videos. Use videos for mixed statuses, or post_ids plus status for one status. Rejected videos are disabled upstream by default.
// @Tags review
// @Accept json
// @Produce json
// @Param request body BatchReviewRequest true "Batch review request"
// @Success 200 {object} BatchReviewResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /review/batch [post]
func BatchReview(c *gin.Context) {
	var req BatchReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	items := make([]BatchReviewItem, 0, len(req.Videos)+len(req.PostIDs))
	items = append(items, req.Videos...)
	if len(req.PostIDs) > 0 {
		status := strings.ToLower(strings.TrimSpace(req.Status))
		if !isValidReviewStatus(status) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "status must be approved or rejected when post_ids is used"})
			return
		}
		for _, postID := range req.PostIDs {
			items = append(items, BatchReviewItem{PostID: postID, Status: status})
		}
	}
	if len(items) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "videos or post_ids is required"})
		return
	}

	seen := make(map[int]struct{}, len(items))
	normalized := make([]BatchReviewItem, 0, len(items))
	for _, item := range items {
		item.Status = strings.ToLower(strings.TrimSpace(item.Status))
		if item.PostID < 1 {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "post_id must be greater than zero"})
			return
		}
		if item.Status == "" {
			item.Status = strings.ToLower(strings.TrimSpace(req.Status))
		}
		if !isValidReviewStatus(item.Status) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "each video status must be approved or rejected"})
			return
		}
		if _, exists := seen[item.PostID]; exists {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "duplicate post_id: " + strconv.Itoa(item.PostID)})
			return
		}
		seen[item.PostID] = struct{}{}
		normalized = append(normalized, item)
	}

	disableRejected := true
	if req.DisableRejected != nil {
		disableRejected = *req.DisableRejected
	}

	results := make([]BatchReviewResult, 0, len(normalized))
	successful := make([]BatchReviewItem, 0, len(normalized))
	successCount := 0
	failedCount := 0
	disabledCount := 0
	for _, item := range normalized {
		result := BatchReviewResult{PostID: item.PostID, Status: item.Status}
		if item.Status == reviewStatusRejected && disableRejected {
			result.Success, result.Message = disableVideoUpstream(c.Request.Context(), item.PostID)
			result.Disabled = result.Success
			if result.Success {
				disabledCount++
			}
		} else {
			result.Success = true
			result.Message = "ok"
		}

		if result.Success {
			successCount++
			successful = append(successful, item)
		} else {
			failedCount++
		}
		results = append(results, result)
	}

	reviewStateMu.Lock()
	defer reviewStateMu.Unlock()
	state, err := loadReviewState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to load review state: " + err.Error()})
		return
	}
	for _, item := range successful {
		state.Statuses[item.PostID] = item.Status
	}
	normalizeReviewState(state)
	if err := saveReviewState(state); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to save review state: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, BatchReviewResponse{
		Success:  successCount,
		Failed:   failedCount,
		Total:    len(normalized),
		Disabled: disabledCount,
		Results:  results,
		State:    state,
	})
}

// ClearReviewState godoc
// @Summary Clear review state
// @Description Clear all reviewed video IDs from server-side JSON file
// @Tags review
// @Produce json
// @Success 200 {object} ReviewState
// @Failure 500 {object} ErrorResponse
// @Router /review/state [delete]
func ClearReviewState(c *gin.Context) {
	reviewStateMu.Lock()
	defer reviewStateMu.Unlock()

	state := newReviewState()
	if err := saveReviewState(state); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to save review state: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, state)
}
