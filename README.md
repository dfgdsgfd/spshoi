# Video Center API

A Go-Gin based API for video center management. Provides endpoints to list videos and batch toggle video enable/disable status.

## Features

- **GET /api/videos** — Fetch video list with pagination, search, and sort order
- **POST /api/videos/batch-toggle** — Batch enable/disable videos
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
# Optional: override the upstream base URL (defaults to https://v.yuelk.com)
# export VIDEO_API_BASE_URL="https://v.yuelk.com"

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

## Testing

```bash
go test ./... -v
```