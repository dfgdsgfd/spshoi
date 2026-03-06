package handlers

import (
	"embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed templates/review.html
var reviewHTML embed.FS

// ReviewPage serves the embedded video review HTML page
func ReviewPage(c *gin.Context) {
	data, err := reviewHTML.ReadFile("templates/review.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load review page")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}
