package handlers

import (
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
				"preview_video_url": "https://edgecdn2-tc.yuelk.com:30086/video/preview/index.m3u8?token=abc"
			},
			{
				"id": 2,
				"title": "Video 2",
				"preview_video_url": "https://edgecdn2-tc.yuelk.com:30086/video/preview2/index.m3u8?token=def"
			}
		],
		"total_posts": 2,
		"total_pages": 1
	}`

	result := rewriteVideoURLs([]byte(input))
	resultStr := string(result)

	// Original CDN URLs should NOT be present
	if strings.Contains(resultStr, "edgecdn2-tc.yuelk.com:30086") {
		t.Error("expected CDN URLs to be rewritten, but found original URL in output")
	}

	// Proxy URLs should be present
	if !strings.Contains(resultStr, "/api/proxy/video?url=") {
		t.Error("expected proxy URLs in output")
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
