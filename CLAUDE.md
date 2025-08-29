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
- `EDGEGAP_VERSION` - Version to deploy (required if dynamic versioning is disabled)
- `EDGEGAP_PORT_NAME` - Port name to expose to game clients
- `NAKAMA_ACCESS_URL` - Nakama API URL (must use https://)

Optional:
- `EDGEGAP_DYNAMIC_VERSIONING` - Enable dynamic versioning from storage (true/false, default: false)

### Dynamic Versioning
When enabled, the plugin reads deployment versions from Nakama storage (`system/edgegap_version`) instead of environment variables. This allows runtime version updates without service restarts.

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

## Testing Approach

The project uses Go's standard testing framework. Tests should be placed alongside source files with `_test.go` suffix.