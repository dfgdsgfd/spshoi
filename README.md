# Video Center API

A Go-Gin based API for video center management. Provides endpoints to list videos and batch toggle video enable/disable status.

## Features

- **GET /api/videos** — Fetch video list with pagination, search, and sort order; use `all=true` to load all upstream pages at once
- **POST /api/videos/batch-toggle** — Batch enable/disable videos
- **POST /api/review/batch** — Batch save AI or manual review results by video IDs
- **Swagger Docs** — Auto-generated API documentation at `/docs/index.html` (no authentication required)
- **Cross-platform builds** — GitHub Actions CI for Linux and Windows

## Quick Start

```bash
# Install swag CLI (for Swagger doc generation)
go install github.com/swaggo/swag/cmd/swag@v1.16.4

# Generate Swagger docs
swag init --parseDependency

# Build
go build -o video-center .

# Set required environment variables
export VIDEO_API_KEY="your-api-key-here"
# Required: session cookie for the upstream manage API (pyvideo2_session value)
export VIDEO_SESSION_COOKIE="your-session-cookie-here"
# Optional: override the upstream base URL (defaults to https://v2.yuelk.com)
# export VIDEO_API_BASE_URL="https://v2.yuelk.com"

# Run
./video-center
```

The server starts on port `8080` by default. Set the `PORT` environment variable to change it.

## API Documentation

Once the server is running, visit: http://localhost:8080/docs/index.html

## API Endpoints

### GET /api/videos

| Parameter | Type   | Default | Description          |
|-----------|--------|---------|----------------------|
| page      | int    | 1       | Page number          |
| per_page  | int    | 20      | Items per page (1-100) |
| search    | string | (empty) | Search keyword       |
| order     | string | DESC    | Sort order (ASC/DESC) |
| all       | bool   | false   | Aggregate all upstream pages into one response |

### POST /api/videos/batch-toggle

Request body:

```json
{
  "videos": [
    {"post_id": 123, "enable": false},
    {"post_id": 456, "enable": true}
  ]
}
```

### POST /api/review/batch

Use `videos` when each video has its own result:

```json
{
  "videos": [
    {"post_id": 123, "status": "approved"},
    {"post_id": 456, "status": "rejected"}
  ],
  "disable_rejected": true
}
```

For one result applied to many IDs, use `post_ids` plus `status`:

```json
{
  "post_ids": [123, 456, 789],
  "status": "approved"
}
```

`status` accepts `approved` or `rejected`. Rejected videos are disabled upstream by default; set `disable_rejected` to `false` when an AI caller only wants to save the review result.

The review page provides “只显示复审视频” and “只显示被屏蔽视频” filters. Either filter automatically uses `all=true`, so those views are not limited by page number. Video IDs can be entered in batches using commas, spaces, or new lines, then processed with the batch buttons.

## Testing

```bash
go test ./... -v
```
