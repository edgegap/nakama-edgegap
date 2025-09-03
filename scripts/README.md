# Test Scripts

This directory contains utility scripts for testing and managing the Nakama-Edgegap integration.

## Prerequisites

- Docker and Docker Compose installed
- Nakama running locally (`docker compose up -d`)
- HTTP key configured in `local.yml` (default: `testkey123`)

## Available Scripts

### test_get_version.bat
Gets the current Edgegap version configuration from Nakama storage.

```bash
# Usage
./scripts/test_get_version.bat [http_key] [nakama_url]

# Example (using defaults)
./scripts/test_get_version.bat

# Example (custom key and URL)
./scripts/test_get_version.bat "mykey123" "http://localhost:7350"
```

**Response when version is configured:**
```json
{
  "version": "your-version",
  "source": "storage",
  "updated_at": 1756414933
}
```

**Response when no version is set:**
```json
{
  "error": "No Edgegap version configured",
  "message": "Please set version using update_edgegap_version RPC"
}
```

### test_update_version.bat
Updates the Edgegap deployment version in Nakama storage.

```bash
# Usage
./scripts/test_update_version.bat <version> [http_key] [nakama_url]

# Examples
./scripts/test_update_version.bat "v1.0.0"
./scripts/test_update_version.bat "production-v2.5.0"
./scripts/test_update_version.bat "2025-01-04_12-41"
```

**Response:**
```json
{
  "success": true,
  "version": "v1.0.0",
  "message": "Edgegap version updated successfully. Will be used for new deployments immediately."
}
```

## Configuration

The scripts use the following defaults:
- HTTP Key: `testkey123` (matches `local.yml.example`)
- Nakama URL: `http://localhost:7350`

Make sure your `local.yml` has:
```yaml
runtime:
  http_key: "testkey123"
```

## Notes

- The version format is flexible - Edgegap accepts any string as version
- Updates take effect immediately for new deployments
- The HTTP key must match what's configured in Nakama's `runtime.http_key` setting
- Versions are stored in Nakama storage and persist across restarts