package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	url_pkg "net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultBaseURL = "https://v.yuelk.com"
	defaultAPIKey  = "ef13c2bdf8cd8550ed4c37c323a558c9985d6d928d39a3b53bed864460221d56"
)

func getBaseURL() string {
	if v := os.Getenv("VIDEO_API_BASE_URL"); v != "" {
		return v
	}
	return defaultBaseURL
}

func getAPIKey() string {
	if v := os.Getenv("VIDEO_API_KEY"); v != "" {
		return v
	}
	return defaultAPIKey
}

// VideoListResponse represents the response from the video list API
type VideoListResponse struct {
	Posts      []interface{} `json:"posts"`
	Page       int           `json:"page"`
	TotalPages int           `json:"total_pages"`
	Total      int           `json:"total"`
}

// VideoToggleRequest represents a single video toggle item
type VideoToggleRequest struct {
	PostID int  `json:"post_id" binding:"required" example:"1"`
	Enable bool `json:"enable" example:"false"`
}

// BatchToggleRequest represents the request body for batch toggle
type BatchToggleRequest struct {
	Videos []VideoToggleRequest `json:"videos" binding:"required,min=1"`
}

// ToggleResult represents the result of a single toggle operation
type ToggleResult struct {
	PostID  int    `json:"post_id"`
	Enable  bool   `json:"enable"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// BatchToggleResponse represents the response for batch toggle
type BatchToggleResponse struct {
	Success int            `json:"success"`
	Failed  int            `json:"failed"`
	Total   int            `json:"total"`
	Results []ToggleResult `json:"results"`
}

// GetVideoURLRequest represents the request for getting a full video URL
type GetVideoURLRequest struct {
	PostID  int    `json:"post_id" binding:"required" example:"11434"`
	Quality string `json:"quality" example:"720p"`
}

// GetVideoURLResponse represents the response from the get-video-url API
type GetVideoURLResponse struct {
	VideoURL    string  `json:"video_url"`
	SubtitleURL string  `json:"subtitle_url"`
	Quality     string  `json:"quality"`
	PostID      int     `json:"post_id"`
	Duration    float64 `json:"duration"`
}

// GetVideoURL godoc
// @Summary Get full video URL
// @Description Fetch the full (non-preview) video URL from the upstream API for a given post ID and quality
// @Tags videos
// @Accept json
// @Produce json
// @Param request body GetVideoURLRequest true "Post ID and quality"
// @Success 200 {object} GetVideoURLResponse
// @Failure 400 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Router /videos/get-url [post]
func GetVideoURL(c *gin.Context) {
	var req GetVideoURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	if req.Quality == "" {
		req.Quality = "720p"
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"post_id": req.PostID,
		"quality": req.Quality,
	})

	url := fmt.Sprintf("%s/pyvideo2/get-video-url", getBaseURL())
	client := &http.Client{Timeout: 15 * time.Second}
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("X-API-KEY", getAPIKey())

	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to fetch video URL from upstream"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to read upstream response"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, ErrorResponse{Error: fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(body))})
		return
	}

	// Parse the upstream response and rewrite the video URL
	var upstream map[string]interface{}
	if err := json.Unmarshal(body, &upstream); err != nil {
		c.Data(resp.StatusCode, "application/json", body)
		return
	}

	if videoURL, ok := upstream["video_url"].(string); ok && videoURL != "" {
		upstream["video_url"] = makeProxyURL(replaceVideoHost(html.UnescapeString(videoURL)))
	}
	if subtitleURL, ok := upstream["subtitle_url"].(string); ok && subtitleURL != "" {
		upstream["subtitle_url"] = html.UnescapeString(subtitleURL)
	}

	rewritten, err := json.Marshal(upstream)
	if err != nil {
		c.Data(resp.StatusCode, "application/json", body)
		return
	}
	c.Data(http.StatusOK, "application/json", rewritten)
}

// BatchDisableRequest represents the request body for batch disable
type BatchDisableRequest struct {
	PostIDs []int `json:"post_ids" binding:"required,min=1" example:"1,2,3"`
}

// DisableResult represents the result of a single disable operation
type DisableResult struct {
	PostID  int    `json:"post_id"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// BatchDisableResponse represents the response for batch disable
type BatchDisableResponse struct {
	Disabled int             `json:"disabled"`
	Failed   int             `json:"failed"`
	Total    int             `json:"total"`
	Results  []DisableResult `json:"results"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// GetVideos godoc
// @Summary Get video center list
// @Description Fetch video list from the upstream API with pagination, search, and sorting options
// @Tags videos
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1) minimum(1)
// @Param per_page query int false "Items per page" default(20) minimum(1) maximum(100)
// @Param search query string false "Search keyword"
// @Param order query string false "Sort order" Enums(ASC, DESC) default(DESC)
// @Success 200 {object} VideoListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Router /videos [get]
func GetVideos(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	search := c.Query("search")
	order := strings.ToUpper(c.DefaultQuery("order", "DESC"))
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}

	url := fmt.Sprintf("%s/pyvideo2/api/get_posts?page=%d&per_page=%d&sort_order=%s",
		getBaseURL(), page, perPage, order)
	if search != "" {
		url += "&search=" + url_pkg.QueryEscape(search)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-KEY", getAPIKey())

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to fetch data from upstream"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to read upstream response"})
		return
	}

	// Rewrite video URLs to go through our proxy so browsers can access them
	rewritten := rewriteVideoURLs(body)
	c.Data(resp.StatusCode, "application/json", rewritten)
}

// BatchToggleVideos godoc
// @Summary Batch toggle video enable/disable
// @Description Toggle enable/disable status for multiple videos at once via the upstream admin API
// @Tags videos
// @Accept json
// @Produce json
// @Param request body BatchToggleRequest true "List of videos to toggle"
// @Success 200 {object} BatchToggleResponse
// @Failure 400 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Router /videos/batch-toggle [post]
func BatchToggleVideos(c *gin.Context) {
	var req BatchToggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	client := &http.Client{Timeout: 15 * time.Second}
	results := make([]ToggleResult, 0, len(req.Videos))
	successCount := 0
	failedCount := 0

	for _, video := range req.Videos {
		result := ToggleResult{
			PostID: video.PostID,
			Enable: video.Enable,
		}

		payload, _ := json.Marshal(map[string]interface{}{
			"post_id": video.PostID,
			"enable":  video.Enable,
		})

		url := fmt.Sprintf("%s/pyvideo2/api/admin/video-enable-toggle", getBaseURL())
		httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			result.Success = false
			result.Message = "failed to create request"
			failedCount++
			results = append(results, result)
			continue
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")
		httpReq.Header.Set("X-API-KEY", getAPIKey())

		resp, err := client.Do(httpReq)
		if err != nil {
			result.Success = false
			result.Message = "failed to call upstream API"
			failedCount++
			results = append(results, result)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			result.Success = true
			result.Message = "ok"
			successCount++
		} else {
			result.Success = false
			result.Message = fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(body))
			failedCount++
		}

		results = append(results, result)
	}

	c.JSON(http.StatusOK, BatchToggleResponse{
		Success: successCount,
		Failed:  failedCount,
		Total:   len(req.Videos),
		Results: results,
	})
}

// BatchDisableVideos godoc
// @Summary Batch disable videos
// @Description Batch disable multiple videos by calling the upstream video-enable-toggle API with enable=false for each post ID
// @Tags videos
// @Accept json
// @Produce json
// @Param request body BatchDisableRequest true "List of post IDs to disable"
// @Success 200 {object} BatchDisableResponse
// @Failure 400 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Router /videos/batch-disable [post]
func BatchDisableVideos(c *gin.Context) {
	var req BatchDisableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	client := &http.Client{Timeout: 15 * time.Second}
	results := make([]DisableResult, 0, len(req.PostIDs))
	disabledCount := 0
	failedCount := 0

	for _, postID := range req.PostIDs {
		result := DisableResult{
			PostID: postID,
		}

		payload, _ := json.Marshal(map[string]interface{}{
			"post_id": postID,
			"enable":  false,
		})

		url := fmt.Sprintf("%s/pyvideo2/api/admin/video-enable-toggle", getBaseURL())
		httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			result.Success = false
			result.Message = "failed to create request"
			failedCount++
			results = append(results, result)
			continue
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")
		httpReq.Header.Set("X-API-KEY", getAPIKey())

		resp, err := client.Do(httpReq)
		if err != nil {
			result.Success = false
			result.Message = "failed to call upstream API"
			failedCount++
			results = append(results, result)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			result.Success = true
			result.Message = "disabled"
			disabledCount++
		} else {
			result.Success = false
			result.Message = fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(body))
			failedCount++
		}

		results = append(results, result)
	}

	c.JSON(http.StatusOK, BatchDisableResponse{
		Disabled: disabledCount,
		Failed:   failedCount,
		Total:    len(req.PostIDs),
		Results:  results,
	})
}
