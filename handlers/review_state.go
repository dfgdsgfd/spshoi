package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
)

const defaultReviewStatePath = "review_state.json"

// ReviewState stores the list of reviewed video IDs
type ReviewState struct {
	ReviewedIDs []int `json:"reviewed_ids"`
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
			return &ReviewState{ReviewedIDs: []int{}}, nil
		}
		return nil, err
	}
	var state ReviewState
	if err := json.Unmarshal(data, &state); err != nil {
		return &ReviewState{ReviewedIDs: []int{}}, nil
	}
	if state.ReviewedIDs == nil {
		state.ReviewedIDs = []int{}
	}
	return &state, nil
}

func saveReviewState(state *ReviewState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getReviewStatePath(), data, 0644)
}

// GetReviewState godoc
// @Summary Get review state
// @Description Returns the list of reviewed video IDs stored in server-side JSON file
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
// @Description Add a video ID to the reviewed list in server-side JSON file
// @Tags review
// @Accept json
// @Produce json
// @Param request body object true "Post ID to add" example({"post_id": 12345})
// @Success 200 {object} ReviewState
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /review/state [post]
func AddReviewedID(c *gin.Context) {
	var req struct {
		PostID int `json:"post_id" binding:"required"`
	}
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

	// Check if already exists
	for _, id := range state.ReviewedIDs {
		if id == req.PostID {
			c.JSON(http.StatusOK, state)
			return
		}
	}

	state.ReviewedIDs = append(state.ReviewedIDs, req.PostID)
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

	state := &ReviewState{ReviewedIDs: []int{}}
	if err := saveReviewState(state); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to save review state: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, state)
}
