package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
)

const defaultPageCachePath = "page_cache.json"

// PageCacheState stores cached page data keyed by page number
type PageCacheState struct {
	Pages map[string]json.RawMessage `json:"pages"`
}

var pageCacheMu sync.Mutex

func getPageCachePath() string {
	if v := os.Getenv("PAGE_CACHE_PATH"); v != "" {
		return v
	}
	return defaultPageCachePath
}

func loadPageCache() (*PageCacheState, error) {
	path := getPageCachePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &PageCacheState{Pages: map[string]json.RawMessage{}}, nil
		}
		return nil, err
	}
	var state PageCacheState
	if err := json.Unmarshal(data, &state); err != nil {
		return &PageCacheState{Pages: map[string]json.RawMessage{}}, nil
	}
	if state.Pages == nil {
		state.Pages = map[string]json.RawMessage{}
	}
	return &state, nil
}

func savePageCache(state *PageCacheState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(getPageCachePath(), data, 0600)
}

// GetPageCache godoc
// @Summary Get cached page data
// @Description Returns all cached video page data stored in server-side JSON file
// @Tags review
// @Produce json
// @Success 200 {object} PageCacheState
// @Failure 500 {object} ErrorResponse
// @Router /review/pages [get]
func GetPageCache(c *gin.Context) {
	pageCacheMu.Lock()
	defer pageCacheMu.Unlock()

	state, err := loadPageCache()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to load page cache: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, state)
}

// SavePageCache godoc
// @Summary Save a page of video data
// @Description Save a page's API response data to server-side JSON file
// @Tags review
// @Accept json
// @Produce json
// @Param request body object true "Page number and data" example({"page": "1", "data": {}})
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /review/pages [post]
func SavePageCache(c *gin.Context) {
	var req struct {
		Page string          `json:"page" binding:"required"`
		Data json.RawMessage `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	pageCacheMu.Lock()
	defer pageCacheMu.Unlock()

	state, err := loadPageCache()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to load page cache: " + err.Error()})
		return
	}

	state.Pages[req.Page] = req.Data
	if err := savePageCache(state); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to save page cache: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ClearPageCache godoc
// @Summary Clear cached page data
// @Description Clear all cached video page data from server-side JSON file
// @Tags review
// @Produce json
// @Success 200 {object} PageCacheState
// @Failure 500 {object} ErrorResponse
// @Router /review/pages [delete]
func ClearPageCache(c *gin.Context) {
	pageCacheMu.Lock()
	defer pageCacheMu.Unlock()

	state := &PageCacheState{Pages: map[string]json.RawMessage{}}
	if err := savePageCache(state); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to save page cache: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, state)
}
