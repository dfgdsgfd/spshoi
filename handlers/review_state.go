package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
)

const defaultReviewStatePath = "review_state.json"

const (
	reviewStatusReviewed = "reviewed"
	reviewStatusApproved = "approved"
	reviewStatusRejected = "rejected"
	reviewStatusRecheck  = "recheck"
)

// ReviewState stores the review result for each video. ReviewedIDs is kept for
// compatibility with existing state files and clients; Statuses holds the
// detailed result used by the review UI.
type ReviewState struct {
	ReviewedIDs []int          `json:"reviewed_ids"`
	Statuses    map[int]string `json:"statuses"`
}

// ReviewStatusRequest updates one video's detailed review status.
type ReviewStatusRequest struct {
	PostID int    `json:"post_id" binding:"required" example:"12345"`
	Status string `json:"status" enums:"reviewed,approved,rejected,recheck" example:"approved"`
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

	// State files created before detailed statuses existed only contain
	// ReviewedIDs. Preserve those records as generic completed reviews.
	for _, id := range state.ReviewedIDs {
		if _, ok := state.Statuses[id]; !ok {
			state.Statuses[id] = reviewStatusReviewed
		}
	}

	// A recheck is deliberately not counted as completed. Rebuild the legacy
	// list from statuses so it always agrees with the displayed count.
	completed := make([]int, 0, len(state.Statuses))
	for id, status := range state.Statuses {
		if status != reviewStatusRecheck {
			completed = append(completed, id)
		}
	}
	state.ReviewedIDs = completed
}

func isValidReviewStatus(status string) bool {
	switch status {
	case reviewStatusReviewed, reviewStatusApproved, reviewStatusRejected, reviewStatusRecheck:
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
// @Description Save a video's review status. Omit status to mark it as a generic completed review.
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

	if req.Status == "" {
		req.Status = reviewStatusReviewed
	}
	if !isValidReviewStatus(req.Status) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid review status"})
		return
	}

	state.Statuses[req.PostID] = req.Status
	normalizeReviewState(state)
	if err := saveReviewState(state); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to save review state: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, state)
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
