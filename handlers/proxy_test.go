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
		{"https://spcs.yuelk.com:29443/video/index.m3u8", true},
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

func TestRewriteM3U8_WithEncryptionKey(t *testing.T) {
	baseURL := "https://spcs.yuelk.com:29443/wp-content/uploads/video/2026-02-04/72035dfd3e_RTmd7E/720p_c57bc6/index.m3u8"

	input := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:8
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-KEY:METHOD=AES-128,URI="enc.key",IV=0x20481f7985eee23400db1e486481f246
#EXTINF:8.333333,
segment_000.ts
#EXTINF:8.333333,
segment_001.ts
#EXT-X-ENDLIST`

	result := rewriteM3U8(input, baseURL)
	lines := strings.Split(result, "\n")

	// Check that enc.key URI is rewritten to proxy URL
	foundKey := false
	for _, line := range lines {
		if strings.Contains(line, "EXT-X-KEY") {
			foundKey = true
			if !strings.Contains(line, "URI=\""+proxyVideoPath) {
				t.Errorf("expected EXT-X-KEY URI to be proxied, got: %s", line)
			}
			// Verify the resolved URL contains the correct base path
			idx := strings.Index(line, "URI=\""+proxyVideoPath)
			if idx != -1 {
				start := idx + 5 + len(proxyVideoPath)
				end := strings.Index(line[start:], "\"")
				if end != -1 {
					encoded := line[start : start+end]
					decoded, err := url.QueryUnescape(encoded)
					if err != nil {
						t.Fatalf("failed to decode enc.key URL: %v", err)
					}
					if !strings.HasSuffix(decoded, "/enc.key") {
						t.Errorf("expected enc.key in decoded URL, got: %s", decoded)
					}
				}
			}
		}
	}
	if !foundKey {
		t.Error("expected EXT-X-KEY line in output")
	}

	// Check that segment URLs are also proxied
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(trimmed, proxyVideoPath) {
			t.Errorf("expected segment URL to be proxied, got: %s", trimmed)
		}
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

	// Video URLs should now be proxied (wrapped in /api/proxy/video?url=...)
	if !strings.Contains(resultStr, "/api/proxy/video?url=") {
		t.Error("expected video proxy URLs in output")
	}

	// The proxied URL should contain the play base URL (not CDN)
	var data map[string]interface{}
	json.Unmarshal(result, &data)
	posts := data["posts"].([]interface{})
	for _, p := range posts {
		pm := p.(map[string]interface{})
		videoURL := pm["preview_video_url"].(string)
		if !strings.HasPrefix(videoURL, proxyVideoPath) {
			t.Errorf("expected preview_video_url to start with proxy path, got: %s", videoURL)
		}
		// Decode and check the internal URL uses play base
		encoded := strings.TrimPrefix(videoURL, proxyVideoPath)
		decoded, err := url.QueryUnescape(encoded)
		if err != nil {
			t.Errorf("failed to decode proxy URL: %v", err)
		}
		if !strings.HasPrefix(decoded, getVideoPlayBaseURL()) {
			t.Errorf("expected proxied URL to use play base URL, got: %s", decoded)
		}
	}

	// Image URLs should use the image proxy path
	if !strings.Contains(resultStr, "/api/proxy/image?url=") {
		t.Error("expected image proxy URLs in output")
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

func TestRewriteVideoURLs_WithVideoURL(t *testing.T) {
	input := `{
		"posts": [
			{
				"id": 1,
				"title": "Test Video",
				"preview_video_url": "https://edgecdn2-tc.yuelk.com:30086/video/preview/index.m3u8?token=abc",
				"video_url": "https://edgecdn2-tc.yuelk.com:30086/video/720p_abc123/index.m3u8?token=def",
				"first_image": "https://v.yuelk.com/pima/wp-content/uploads/video/2025-11-13/abc/vod.webp"
			}
		]
	}`

	result := rewriteVideoURLs([]byte(input))
	var data map[string]interface{}
	json.Unmarshal(result, &data)
	posts := data["posts"].([]interface{})
	pm := posts[0].(map[string]interface{})

	// video_url should be rewritten through proxy
	videoURL, ok := pm["video_url"].(string)
	if !ok || videoURL == "" {
		t.Fatal("expected video_url to be present")
	}
	if !strings.HasPrefix(videoURL, proxyVideoPath) {
		t.Errorf("expected video_url to start with proxy path, got: %s", videoURL)
	}

	// Decode and check the internal URL uses play base
	encoded := strings.TrimPrefix(videoURL, proxyVideoPath)
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		t.Fatalf("failed to decode video_url: %v", err)
	}
	if !strings.HasPrefix(decoded, getVideoPlayBaseURL()) {
		t.Errorf("expected proxied video_url to use play base URL, got: %s", decoded)
	}
	if !strings.Contains(decoded, "720p_abc123") {
		t.Errorf("expected 720p path in video_url, got: %s", decoded)
	}

	// preview_video_url should also still be rewritten
	previewURL := pm["preview_video_url"].(string)
	if !strings.HasPrefix(previewURL, proxyVideoPath) {
		t.Errorf("expected preview_video_url to start with proxy path, got: %s", previewURL)
	}
}

func TestRewriteVideoURLs_WithP720Path(t *testing.T) {
	input := `{
		"posts": [
			{
				"id": 1,
				"title": "Test Video",
				"preview_video_url": "https://edgecdn2-tc.yuelk.com:30086/video/preview/index.m3u8?token=abc",
				"video_url": "https://edgecdn2-tc.yuelk.com:30086/video/old/index.m3u8?token=def",
				"p720_path": "video/2026-05-24/2c61e89ed6_nrSXt1/720p_5f0e3b/index.m3u8"
			}
		]
	}`

	result := rewriteVideoURLs([]byte(input))
	var data map[string]interface{}
	json.Unmarshal(result, &data)
	posts := data["posts"].([]interface{})
	pm := posts[0].(map[string]interface{})

	videoURL := pm["video_url"].(string)
	if !strings.HasPrefix(videoURL, proxyVideoPath) {
		t.Errorf("expected video_url to start with proxy path, got: %s", videoURL)
	}

	encoded := strings.TrimPrefix(videoURL, proxyVideoPath)
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		t.Fatalf("failed to decode video_url: %v", err)
	}
	expected := getVideoPlayBaseURL() + "/video/2026-05-24/2c61e89ed6_nrSXt1/720p_5f0e3b/index.m3u8"
	if decoded != expected {
		t.Errorf("expected p720_path-derived URL %q, got %q", expected, decoded)
	}
	if strings.Contains(decoded, "/old/") {
		t.Errorf("expected p720_path to take precedence over stale video_url, got %s", decoded)
	}
}

func TestRewriteVideoURLs_WithOriginalPath(t *testing.T) {
	input := `{
		"posts": [
			{
				"id": 1,
				"title": "Test Video",
				"preview_video_url": "https://edgecdn2-tc.yuelk.com:30086/video/preview/index.m3u8?token=abc",
				"original_path": "video/2026-05-24/2c61e89ed6_nrSXt1/default_5b41c0/index.m3u8"
			}
		]
	}`

	result := rewriteVideoURLs([]byte(input))
	var data map[string]interface{}
	json.Unmarshal(result, &data)
	posts := data["posts"].([]interface{})
	pm := posts[0].(map[string]interface{})

	videoURL := pm["video_url"].(string)
	if !strings.HasPrefix(videoURL, proxyVideoPath) {
		t.Errorf("expected video_url to start with proxy path, got: %s", videoURL)
	}

	decoded, err := url.QueryUnescape(strings.TrimPrefix(videoURL, proxyVideoPath))
	if err != nil {
		t.Fatalf("failed to decode video_url: %v", err)
	}
	expected := getVideoPlayBaseURL() + "/video/2026-05-24/2c61e89ed6_nrSXt1/default_5b41c0/index.m3u8"
	if decoded != expected {
		t.Errorf("expected original_path-derived URL %q, got %q", expected, decoded)
	}
}

func TestRewriteVideoURLs_OriginalPathTakesPrecedence(t *testing.T) {
	input := `{
		"posts": [
			{
				"id": 1,
				"title": "Test Video",
				"preview_video_url": "https://edgecdn2-tc.yuelk.com:30086/video/preview/index.m3u8?token=abc",
				"original_path": "video/2026-05-24/2c61e89ed6_nrSXt1/default_5b41c0/index.m3u8",
				"p720_path": "video/2026-05-24/2c61e89ed6_nrSXt1/720p_5f0e3b/index.m3u8"
			}
		]
	}`

	result := rewriteVideoURLs([]byte(input))
	var data map[string]interface{}
	json.Unmarshal(result, &data)
	posts := data["posts"].([]interface{})
	pm := posts[0].(map[string]interface{})

	decoded, err := url.QueryUnescape(strings.TrimPrefix(pm["video_url"].(string), proxyVideoPath))
	if err != nil {
		t.Fatalf("failed to decode video_url: %v", err)
	}
	expected := getVideoPlayBaseURL() + "/video/2026-05-24/2c61e89ed6_nrSXt1/default_5b41c0/index.m3u8"
	if decoded != expected {
		t.Errorf("expected original_path to take precedence, got %q", decoded)
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

func TestBuildVideoURLFromPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "relative p720 path",
			input:    "video/2026-05-24/2c61e89ed6_nrSXt1/720p_5f0e3b/index.m3u8",
			expected: getVideoPlayBaseURL() + "/video/2026-05-24/2c61e89ed6_nrSXt1/720p_5f0e3b/index.m3u8",
		},
		{
			name:     "relative p720 path with leading slash",
			input:    "/video/2026-05-24/2c61e89ed6_nrSXt1/720p_5f0e3b/index.m3u8",
			expected: getVideoPlayBaseURL() + "/video/2026-05-24/2c61e89ed6_nrSXt1/720p_5f0e3b/index.m3u8",
		},
		{
			name:     "absolute CDN URL",
			input:    "https://edgecdn2-tc.yuelk.com:30086/video/2026-05-24/2c61e89ed6_nrSXt1/720p_5f0e3b/index.m3u8",
			expected: getVideoPlayBaseURL() + "/video/2026-05-24/2c61e89ed6_nrSXt1/720p_5f0e3b/index.m3u8",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildVideoURLFromPath(tt.input)
			if got != tt.expected {
				t.Errorf("buildVideoURLFromPath(%q) = %q, want %q", tt.input, got, tt.expected)
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

func TestGetVideoURL_WithP720Path(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusForbidden)
	}))
	defer upstream.Close()

	t.Setenv("VIDEO_API_BASE_URL", upstream.URL)

	r := setupRouter()
	reqBody := `{
		"post_id": 123,
		"quality": "720p",
		"p720_path": "video/2026-05-24/2c61e89ed6_nrSXt1/720p_5f0e3b/index.m3u8"
	}`
	req, _ := http.NewRequest(http.MethodPost, "/api/videos/get-url", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if upstreamCalls != 0 {
		t.Errorf("expected no upstream calls when p720_path is present, got %d", upstreamCalls)
	}

	var resp GetVideoURLResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.VideoURL == "" || !strings.HasPrefix(resp.VideoURL, proxyVideoPath) {
		t.Fatalf("expected proxied video_url, got %q", resp.VideoURL)
	}

	decoded, err := url.QueryUnescape(strings.TrimPrefix(resp.VideoURL, proxyVideoPath))
	if err != nil {
		t.Fatalf("failed to decode video_url: %v", err)
	}
	expected := getVideoPlayBaseURL() + "/video/2026-05-24/2c61e89ed6_nrSXt1/720p_5f0e3b/index.m3u8"
	if decoded != expected {
		t.Errorf("expected %q, got %q", expected, decoded)
	}
}

func TestGetVideoURL_WithOriginalPath(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusForbidden)
	}))
	defer upstream.Close()

	t.Setenv("VIDEO_API_BASE_URL", upstream.URL)

	r := setupRouter()
	reqBody := `{
		"post_id": 123,
		"quality": "720p",
		"original_path": "video/2026-05-24/2c61e89ed6_nrSXt1/default_5b41c0/index.m3u8"
	}`
	req, _ := http.NewRequest(http.MethodPost, "/api/videos/get-url", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if upstreamCalls != 0 {
		t.Errorf("expected no upstream calls when original_path is present, got %d", upstreamCalls)
	}

	var resp GetVideoURLResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	decoded, err := url.QueryUnescape(strings.TrimPrefix(resp.VideoURL, proxyVideoPath))
	if err != nil {
		t.Fatalf("failed to decode video_url: %v", err)
	}
	expected := getVideoPlayBaseURL() + "/video/2026-05-24/2c61e89ed6_nrSXt1/default_5b41c0/index.m3u8"
	if decoded != expected {
		t.Errorf("expected %q, got %q", expected, decoded)
	}
}

func TestGetVideoURL_OriginalPathTakesPrecedence(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusForbidden)
	}))
	defer upstream.Close()

	t.Setenv("VIDEO_API_BASE_URL", upstream.URL)

	r := setupRouter()
	reqBody := `{
		"post_id": 123,
		"quality": "720p",
		"original_path": "video/2026-05-24/2c61e89ed6_nrSXt1/default_5b41c0/index.m3u8",
		"p720_path": "video/2026-05-24/2c61e89ed6_nrSXt1/720p_5f0e3b/index.m3u8"
	}`
	req, _ := http.NewRequest(http.MethodPost, "/api/videos/get-url", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if upstreamCalls != 0 {
		t.Errorf("expected no upstream calls when playable paths are present, got %d", upstreamCalls)
	}

	var resp GetVideoURLResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	decoded, err := url.QueryUnescape(strings.TrimPrefix(resp.VideoURL, proxyVideoPath))
	if err != nil {
		t.Fatalf("failed to decode video_url: %v", err)
	}
	expected := getVideoPlayBaseURL() + "/video/2026-05-24/2c61e89ed6_nrSXt1/default_5b41c0/index.m3u8"
	if decoded != expected {
		t.Errorf("expected original_path to take precedence, got %q", decoded)
	}
}
