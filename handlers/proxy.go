package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	proxyVideoPath          = "/api/proxy/video?url="
	proxyImagePath          = "/api/proxy/image?url="
	defaultVideoPlayBaseURL = "http://192.168.80.101:7896"
	defaultImageAPIKey      = "co0slUKg6gA89AiV9oWbKqacVk0yph67"
)

// cdnHosts are upstream CDN hosts whose scheme+host+port should be replaced
// with the video play base URL for direct client access.
var cdnHosts = []string{
	"edgecdn2-tc.yuelk.com:30086",
	"edgeone-cdn.yuelk.com",
}

// staticAllowedProxyHosts are upstream hosts that can always be proxied
var staticAllowedProxyHosts = []string{
	"edgecdn2-tc.yuelk.com",
	"edgeone-cdn.yuelk.com",
	"v.yuelk.com",
}

// getAllowedProxyHosts returns the list of allowed proxy hosts, including the
// configured video play base URL host so proxied m3u8 content can be fetched.
func getAllowedProxyHosts() []string {
	hosts := make([]string, len(staticAllowedProxyHosts))
	copy(hosts, staticAllowedProxyHosts)
	playBase := getVideoPlayBaseURL()
	if u, err := url.Parse(playBase); err == nil && u.Hostname() != "" {
		hosts = append(hosts, u.Hostname())
	}
	return hosts
}

func getVideoPlayBaseURL() string {
	if v := os.Getenv("VIDEO_PLAY_BASE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultVideoPlayBaseURL
}

func getImageAPIKey() string {
	if v := os.Getenv("IMAGE_API_KEY"); v != "" {
		return v
	}
	return defaultImageAPIKey
}

func isAllowedProxyHost(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := parsedURL.Hostname()
	for _, allowed := range getAllowedProxyHosts() {
		if host == allowed {
			return true
		}
	}
	return false
}

// replaceVideoHost replaces known CDN hosts in a video URL with the
// configured video play base URL so clients can access videos directly.
func replaceVideoHost(videoURL string) string {
	playBase := getVideoPlayBaseURL()
	for _, cdnHost := range cdnHosts {
		// Try both https and http schemes
		for _, scheme := range []string{"https://", "http://"} {
			prefix := scheme + cdnHost
			if strings.HasPrefix(videoURL, prefix) {
				return playBase + videoURL[len(prefix):]
			}
		}
	}
	return videoURL
}

func makeProxyURL(originalURL string) string {
	return proxyVideoPath + url.QueryEscape(originalURL)
}

func makeImageProxyURL(originalURL string) string {
	return proxyImagePath + url.QueryEscape(originalURL)
}

// ProxyVideo godoc
// @Summary Proxy video content
// @Description Proxies video content (m3u8 playlists and ts segments) from upstream CDN. Rewrites URLs in m3u8 playlists to route through the proxy.
// @Tags videos
// @Produce octet-stream
// @Param url query string true "Upstream video URL to proxy"
// @Success 200
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Router /proxy/video [get]
func ProxyVideo(c *gin.Context) {
	rawURL := c.Query("url")
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "missing url parameter"})
		return
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid url"})
		return
	}

	if !isAllowedProxyHost(rawURL) {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "host not allowed"})
		return
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, rawURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to fetch video content"})
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")

	// Check if this is an m3u8 playlist that needs URL rewriting
	isM3U8 := strings.Contains(contentType, "mpegurl") ||
		strings.Contains(contentType, "m3u8") ||
		strings.HasSuffix(parsedURL.Path, ".m3u8")

	if isM3U8 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to read m3u8 content"})
			return
		}
		rewritten := rewriteM3U8(string(body), rawURL)
		c.Data(resp.StatusCode, "application/vnd.apple.mpegurl", []byte(rewritten))
		return
	}

	// For ts segments and other content, stream through directly
	c.Status(resp.StatusCode)
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	if resp.ContentLength >= 0 {
		c.Header("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	}
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		log.Printf("proxy: error streaming response from %s: %v", parsedURL.Host, err)
	}
}

// rewriteM3U8 rewrites URLs in an m3u8 playlist to go through our proxy
func rewriteM3U8(content string, baseURL string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return content
	}
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Rewrite URI attributes in tags (e.g., EXT-X-MAP, EXT-X-KEY)
		if strings.HasPrefix(trimmed, "#") {
			if strings.Contains(trimmed, "URI=\"") {
				lines[i] = rewriteURIAttribute(trimmed, base)
			}
			continue
		}

		// This is a URL line - resolve and proxy it
		resolved := resolveURL(base, trimmed)
		lines[i] = makeProxyURL(resolved)
	}

	return strings.Join(lines, "\n")
}

func resolveURL(base *url.URL, ref string) string {
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(refURL).String()
}

func rewriteURIAttribute(line string, base *url.URL) string {
	idx := strings.Index(line, "URI=\"")
	if idx == -1 {
		return line
	}
	start := idx + 5
	end := strings.Index(line[start:], "\"")
	if end == -1 {
		return line
	}
	end += start

	uri := line[start:end]
	resolved := resolveURL(base, uri)
	proxied := makeProxyURL(resolved)

	return line[:start] + proxied + line[end:]
}

// rewriteVideoURLs rewrites preview_video_url and first_image fields in the
// upstream API response. Video URLs get their CDN host replaced with the play
// base URL for direct access; cover image URLs are routed through our image proxy.
func rewriteVideoURLs(body []byte) []byte {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return body
	}

	posts, ok := data["posts"].([]interface{})
	if !ok {
		return body
	}

	for _, post := range posts {
		postMap, ok := post.(map[string]interface{})
		if !ok {
			continue
		}
		if videoURL, ok := postMap["preview_video_url"].(string); ok && videoURL != "" {
			postMap["preview_video_url"] = makeProxyURL(replaceVideoHost(videoURL))
		}
		if videoURL, ok := postMap["video_url"].(string); ok && videoURL != "" {
			postMap["video_url"] = makeProxyURL(replaceVideoHost(videoURL))
		}
		if imageURL, ok := postMap["first_image"].(string); ok && imageURL != "" {
			postMap["first_image"] = makeImageProxyURL(imageURL)
		}
	}

	rewritten, err := json.Marshal(data)
	if err != nil {
		return body
	}
	return rewritten
}

// ProxyImage godoc
// @Summary Proxy cover image
// @Description Proxies cover images from upstream, adding the required API key header
// @Tags videos
// @Produce octet-stream
// @Param url query string true "Upstream image URL to proxy"
// @Success 200
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Router /proxy/image [get]
func ProxyImage(c *gin.Context) {
	rawURL := c.Query("url")
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "missing url parameter"})
		return
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid url"})
		return
	}

	if !isAllowedProxyHost(rawURL) {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "host not allowed"})
		return
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, rawURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to create request"})
		return
	}
	req.Header.Set("X-API-Key", getImageAPIKey())

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to fetch image"})
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	c.Status(resp.StatusCode)
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	if resp.ContentLength >= 0 {
		c.Header("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	}
	c.Header("Cache-Control", "public, max-age=86400")
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		log.Printf("proxy: error streaming image from %s: %v", parsedURL.Host, err)
	}
}
