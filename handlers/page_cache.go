package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
)

const defaultPageCachePath = "page_cache.json"

// PageCacheState stores only the current page number
type PageCacheState struct {
	CurrentPage int `json:"current_page"`
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
			return &PageCacheState{CurrentPage: 1}, nil
		}
		return nil, err
	}
	var state PageCacheState
	if err := json.Unmarshal(data, &state); err != nil {
		return &PageCacheState{CurrentPage: 1}, nil
	}
	if state.CurrentPage < 1 {
		state.CurrentPage = 1
	}
	return &state, nil
}

func savePageCacheState(state *PageCacheState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(getPageCachePath(), data, 0600)
}

// GetPageCache godoc
// @Summary Get remembered page number
// @Description Returns the remembered current page number from server-side JSON file
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
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to load page state: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, state)
}

// SavePageCache godoc
// @Summary Save current page number
// @Description Save the current page number to server-side JSON file
// @Tags review
// @Accept json
// @Produce json
// @Param request body object true "Current page number" example({"current_page": 1})
// @Success 200 {object} PageCacheState
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /review/pages [post]
func SavePageCache(c *gin.Context) {
	var req struct {
		CurrentPage int `json:"current_page" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	pageCacheMu.Lock()
	defer pageCacheMu.Unlock()

	state := &PageCacheState{CurrentPage: req.CurrentPage}
	if err := savePageCacheState(state); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to save page state: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, state)
}

// ClearPageCache godoc
// @Summary Reset page number
// @Description Reset the remembered page number back to 1
// @Tags review
// @Produce json
// @Success 200 {object} PageCacheState
// @Failure 500 {object} ErrorResponse
// @Router /review/pages [delete]
func ClearPageCache(c *gin.Context) {
	pageCacheMu.Lock()
	defer pageCacheMu.Unlock()

	state := &PageCacheState{CurrentPage: 1}
	if err := savePageCacheState(state); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to save page state: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, state)
}
