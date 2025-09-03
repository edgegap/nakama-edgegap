# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the Nakama Edgegap integration - a Go plugin for Nakama that enables deployment and management of dedicated game servers on Edgegap's infrastructure. The integration handles fleet management, player connections, and server lifecycle events.

## Architecture

### Core Components

- **Fleet Manager** (`pkg/fleetmanager/`) - Main fleet management implementation
  - `edgegap_manager.go` - Edgegap API client and deployment management
  - `fleet_manager.go` - Nakama fleet manager interface implementation
  - `storage.go` - Persistence layer for instance data
  - `event_manager.go` - Handles server lifecycle events
  - `client_rpc.go` - Client-facing RPC endpoints

- **Main Entry Point** (`main.go`) - Plugin initialization and registration

### Key Interfaces

The plugin implements Nakama's Fleet Manager interface to:
- Create/manage Edgegap deployments
- Track player reservations and connections
- Handle instance lifecycle events (READY, ERROR, STOP)
- Provide RPC endpoints for client operations

## Development Commands

### Build the plugin
```bash
go build --trimpath --buildmode=plugin -o ./backend.so main.go
```

### Run local development environment
```bash
# Start Nakama with PostgreSQL
docker compose up --build -d

# Stop the environment
docker compose down
```

### Dependency management
```bash
# Add dependencies
go get <package>

# Update dependencies
go mod tidy

# Vendor dependencies
go mod vendor
```

## Configuration

Required environment variables (set in `local.yml` for local development):
- `EDGEGAP_API_URL` - Edgegap API endpoint
- `EDGEGAP_API_TOKEN` - Authentication token (include "token" prefix)
- `EDGEGAP_APPLICATION` - Application name on Edgegap
- `EDGEGAP_PORT_NAME` - Port name to expose to game clients
- `NAKAMA_ACCESS_URL` - Nakama API URL (must use https://)

Optional:
- `INITIAL_EDGEGAP_VERSION` - Initial version to use if none exists in storage
- `EDGEGAP_VERSION` - (Deprecated) Falls back to this if `INITIAL_EDGEGAP_VERSION` is not set (for backward compatibility)

### Version Management
The plugin reads deployment versions from Nakama storage (`system/edgegap_version`). This allows runtime version updates without service restarts. Use the `update_edgegap_version` RPC to change versions dynamically.

On startup, if no version exists in storage and an initial version is configured (via `INITIAL_EDGEGAP_VERSION` or the deprecated `EDGEGAP_VERSION`), it will be automatically stored for immediate use.

## RPC Endpoints

Client-facing RPCs:
- `instance_create` - Create new game server instance
- `instance_get` - Get instance details
- `instance_list` - List available instances
- `instance_join` - Join existing instance

Server-facing RPCs (called by dedicated game servers):
- `event_deployment` - Deployment status updates
- `event_connection` - Player connection updates
- `event_instance` - Instance lifecycle events (READY/ERROR/STOP)

Version management RPCs (S2S only, require HTTP key):
- `update_edgegap_version` - Update the deployment version
- `get_edgegap_version` - Get current version configuration

## Code Organization

### Error Handling
- Use `errors.Is()` for error comparison instead of string matching
- Package-level error variables are defined (e.g., `ErrorNoVersionFound`)
- Error codes in RPCs map to HTTP status codes via Nakama

### Constants and Strings
- Repeated strings are defined as constants at the package level
- Error messages, log messages, and response fields use constants for consistency
- See `dynamic_version_manager.go` and `edgegap_manager.go` for examples

### Key Files
- `dynamic_version_manager.go` - Handles version management and S2S RPCs
- `configuration.go` - Environment variable configuration with validation
- `storage.go` - Nakama storage operations with defined error types

## Testing Approach

The project uses Go's standard testing framework. Tests should be placed alongside source files with `_test.go` suffix.

### Test Scripts
The `scripts/windows` directory contains Windows batch files for testing version management:
- `test_get_version.bat` - Get current version configuration
- `test_update_version.bat` - Update deployment version
- `test_create_instance.bat` - Test instance creation