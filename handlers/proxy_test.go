package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestMakeProxyURL(t *testing.T) {
	original := "https://edgecdn2-tc.yuelk.com:30086/video/preview/index.m3u8?uid=default&token=abc"
	result := makeProxyURL(original)

	if !strings.HasPrefix(result, proxyVideoPath) {
		t.Errorf("expected proxy URL to start with %s, got %s", proxyVideoPath, result)
	}

	// Decode the url parameter and verify it matches the original
	encoded := strings.TrimPrefix(result, proxyVideoPath)
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		t.Fatalf("failed to decode proxy URL: %v", err)
	}
	if decoded != original {
		t.Errorf("expected decoded URL %q, got %q", original, decoded)
	}
}

func TestIsAllowedProxyHost(t *testing.T) {
	tests := []struct {
		url     string
		allowed bool
	}{
		{"https://edgecdn2-tc.yuelk.com:30086/video/index.m3u8", true},
		{"https://v.yuelk.com/pima/image.webp", true},
		{"https://evil.com/hack", false},
		{"https://example.com/test", false},
		{"not-a-url", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := isAllowedProxyHost(tt.url); got != tt.allowed {
				t.Errorf("isAllowedProxyHost(%q) = %v, want %v", tt.url, got, tt.allowed)
			}
		})
	}
}

func TestRewriteM3U8(t *testing.T) {
	baseURL := "https://edgecdn2-tc.yuelk.com:30086/video/2025-11-13/abc123/preview/index.m3u8?uid=default&token=xyz"

	input := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXTINF:10.000,
seg-0.ts
#EXTINF:10.000,
seg-1.ts
#EXTINF:5.000,
seg-2.ts
#EXT-X-ENDLIST`

	result := rewriteM3U8(input, baseURL)
	lines := strings.Split(result, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Every URL line should now start with the proxy path
		if !strings.HasPrefix(trimmed, proxyVideoPath) {
			t.Errorf("expected URL line to be proxied, got: %s", trimmed)
		}
		// Verify the original segment URL is properly encoded inside
		encoded := strings.TrimPrefix(trimmed, proxyVideoPath)
		decoded, err := url.QueryUnescape(encoded)
		if err != nil {
			t.Errorf("failed to decode proxied URL: %v", err)
		}
		// The decoded URL should be absolute and contain the base path
		if !strings.HasPrefix(decoded, "https://edgecdn2-tc.yuelk.com:30086/video/2025-11-13/abc123/preview/seg-") {
			t.Errorf("expected resolved absolute URL, got: %s", decoded)
		}
	}
}

func TestRewriteM3U8_WithURIAttribute(t *testing.T) {
	baseURL := "https://edgecdn2-tc.yuelk.com:30086/video/preview/index.m3u8?token=abc"

	input := `#EXTM3U
#EXT-X-MAP:URI="init.mp4"
#EXTINF:10.000,
seg-0.ts`

	result := rewriteM3U8(input, baseURL)
	if !strings.Contains(result, "URI=\""+proxyVideoPath) {
		t.Errorf("expected URI attribute to be rewritten, got:\n%s", result)
	}
}

func TestRewriteVideoURLs(t *testing.T) {
	input := `{
		"success": true,
		"posts": [
			{
				"id": 1,
				"title": "Test Video",
				"preview_video_url": "https://edgecdn2-tc.yuelk.com:30086/video/preview/index.m3u8?token=abc",
				"first_image": "https://v.yuelk.com/pima/wp-content/uploads/video/2025-11-13/abc/vod.webp"
			},
			{
				"id": 2,
				"title": "Video 2",
				"preview_video_url": "https://edgecdn2-tc.yuelk.com:30086/video/preview2/index.m3u8?token=def",
				"first_image": "https://v.yuelk.com/pima/wp-content/uploads/video/2025-11-13/def/vod.webp"
			}
		],
		"total_posts": 2,
		"total_pages": 1
	}`

	result := rewriteVideoURLs([]byte(input))
	resultStr := string(result)

	// Original CDN URLs should NOT be present in preview_video_url
	if strings.Contains(resultStr, "edgecdn2-tc.yuelk.com:30086") {
		t.Error("expected CDN URLs to be rewritten, but found original URL in output")
	}

	// Video URLs should use the play base URL
	if !strings.Contains(resultStr, getVideoPlayBaseURL()) {
		t.Errorf("expected video URLs to use play base URL %s", getVideoPlayBaseURL())
	}

	// Image URLs should use the image proxy path
	if !strings.Contains(resultStr, "/api/proxy/image?url=") {
		t.Error("expected image proxy URLs in output")
	}

	// Original image host should NOT be directly present
	var data map[string]interface{}
	json.Unmarshal(result, &data)
	posts := data["posts"].([]interface{})
	for _, p := range posts {
		pm := p.(map[string]interface{})
		imgURL := pm["first_image"].(string)
		if strings.HasPrefix(imgURL, "https://v.yuelk.com") {
			t.Error("expected first_image to be rewritten to proxy URL")
		}
	}
}

func TestRewriteVideoURLs_InvalidJSON(t *testing.T) {
	input := []byte("not json")
	result := rewriteVideoURLs(input)
	if string(result) != string(input) {
		t.Error("expected invalid JSON to be returned as-is")
	}
}

func TestRewriteVideoURLs_NoPosts(t *testing.T) {
	input := []byte(`{"error": "not found"}`)
	result := rewriteVideoURLs(input)
	if string(result) != string(input) {
		t.Error("expected response without posts to be returned as-is")
	}
}

func TestProxyVideo_MissingURL(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/proxy/video", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProxyVideo_InvalidURL(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/proxy/video?url=not-a-valid-url", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProxyVideo_DisallowedHost(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/proxy/video?url="+url.QueryEscape("https://evil.com/hack"), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for disallowed host, got %d", w.Code)
	}
}

func TestProxyVideo_AllowedHostUnreachable(t *testing.T) {
	r := setupRouter()

	// This URL has an allowed host but the CDN is not reachable from test environment
	testURL := "https://edgecdn2-tc.yuelk.com:30086/video/test.ts"
	req, _ := http.NewRequest(http.MethodGet, "/api/proxy/video?url="+url.QueryEscape(testURL), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return 502 (not 400 or 403) since the host is allowed but unreachable
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502 for unreachable allowed host, got %d", w.Code)
	}
}

func TestReplaceVideoHost(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "edgecdn2 host with port",
			input:    "https://edgecdn2-tc.yuelk.com:30086/wp-content/uploads/video/preview/index.m3u8?token=abc",
			expected: getVideoPlayBaseURL() + "/wp-content/uploads/video/preview/index.m3u8?token=abc",
		},
		{
			name:     "edgeone host",
			input:    "https://edgeone-cdn.yuelk.com/wp-content/uploads/video/720p/index.m3u8?token=def",
			expected: getVideoPlayBaseURL() + "/wp-content/uploads/video/720p/index.m3u8?token=def",
		},
		{
			name:     "unknown host unchanged",
			input:    "https://example.com/video.m3u8",
			expected: "https://example.com/video.m3u8",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceVideoHost(tt.input)
			if got != tt.expected {
				t.Errorf("replaceVideoHost(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMakeImageProxyURL(t *testing.T) {
	original := "https://v.yuelk.com/pima/wp-content/uploads/video/2025-11-13/abc/vod.webp"
	result := makeImageProxyURL(original)

	if !strings.HasPrefix(result, proxyImagePath) {
		t.Errorf("expected image proxy URL to start with %s, got %s", proxyImagePath, result)
	}

	encoded := strings.TrimPrefix(result, proxyImagePath)
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		t.Fatalf("failed to decode image proxy URL: %v", err)
	}
	if decoded != original {
		t.Errorf("expected decoded URL %q, got %q", original, decoded)
	}
}

func TestProxyImage_MissingURL(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/proxy/image", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProxyImage_InvalidURL(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/proxy/image?url=not-a-valid-url", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProxyImage_DisallowedHost(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/proxy/image?url="+url.QueryEscape("https://evil.com/image.webp"), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for disallowed host, got %d", w.Code)
	}
}

func TestGetVideoURL_InvalidBody(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodPost, "/api/videos/get-url", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetVideoURL_MissingPostID(t *testing.T) {
	r := setupRouter()

	req, _ := http.NewRequest(http.MethodPost, "/api/videos/get-url", strings.NewReader(`{"quality":"720p"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing post_id, got %d", w.Code)
	}
}
