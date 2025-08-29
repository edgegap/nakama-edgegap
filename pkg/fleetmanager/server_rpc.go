package fleetmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/edgegap/nakama-edgegap/internal/helpers"
	"github.com/heroiclabs/nakama-common/runtime"
	"net/http"
	"strconv"
	"time"
)

const (
	RpcIdUpdateEdgegapVersion = "update_edgegap_version"
	RpcIdGetEdgegapVersion    = "get_edgegap_version"
)

type UpdateEdgegapVersionRequest struct {
	Version string `json:"version"`
}

// updateEdgegapVersion updates the Edgegap deployment version in storage (S2S only)
// Error codes used map to HTTP status codes via Nakama:
// - 3 (INVALID_ARGUMENT) → 400 Bad Request
// - 5 (NOT_FOUND) → 404 Not Found
// - 7 (PERMISSION_DENIED) → 403 Forbidden
// - 9 (FAILED_PRECONDITION) → 400 Bad Request
// - 13 (INTERNAL) → 500 Internal Server Error
func updateEdgegapVersion(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	// This RPC should only be called by servers with HTTP key authentication, not by game clients
	// Nakama automatically validates the HTTP key when the Authorization header is provided
	// If we reach this point with a user ID, it means a client is trying to call this RPC
	if _, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string); ok {
		logger.Warn("Client attempted to call server-to-server RPC for Edgegap version update")
		return "", runtime.NewError("unauthorized: this RPC requires server authentication", 7) // PERMISSION_DENIED
	}

	request := &UpdateEdgegapVersionRequest{}
	if err := json.Unmarshal([]byte(payload), request); err != nil {
		return "", runtime.NewError("invalid payload format", 3) // INVALID_ARGUMENT
	}

	// Validate version is not empty
	if request.Version == "" {
		return "", runtime.NewError("version cannot be empty", 3) // INVALID_ARGUMENT
	}

	// Check if dynamic versioning is enabled
	env, ok := ctx.Value(runtime.RUNTIME_CTX_ENV).(map[string]string)
	if !ok {
		return "", runtime.NewError("failed to get environment context", 13) // INTERNAL
	}

	dynamicVersioning := false
	if dynamicVersioningStr, ok := env["EDGEGAP_DYNAMIC_VERSIONING"]; ok {
		var err error
		dynamicVersioning, err = strconv.ParseBool(dynamicVersioningStr)
		if err != nil {
			// Default to false if parsing fails
			dynamicVersioning = false
		}
	}

	if !dynamicVersioning {
		return "", runtime.NewError("dynamic versioning is not enabled (set EDGEGAP_DYNAMIC_VERSIONING=true)", 9) // FAILED_PRECONDITION
	}

	// Get Edgegap API configuration from environment
	apiUrl := env["EDGEGAP_API_URL"]
	apiToken := env["EDGEGAP_API_TOKEN"]
	application := env["EDGEGAP_APPLICATION"]
	
	if apiUrl == "" || apiToken == "" || application == "" {
		return "", runtime.NewError("missing Edgegap configuration in environment", 13) // INTERNAL
	}

	// Validate the version exists in Edgegap before storing
	apiHelper := helpers.NewAPIClient(apiUrl, apiToken)
	reply, err := apiHelper.Get(fmt.Sprintf("/v1/app/%s/version/%s", application, request.Version))
	if err != nil {
		logger.Error("Failed to validate version with Edgegap API: %v", err)
		return "", runtime.NewError(fmt.Sprintf("failed to validate version with Edgegap API: %v", err), 13) // INTERNAL
	}
	
	if reply.StatusCode != http.StatusOK {
		if reply.StatusCode == http.StatusNotFound {
			return "", runtime.NewError(fmt.Sprintf("version '%s' does not exist for application '%s'", request.Version, application), 5) // NOT_FOUND
		}
		return "", runtime.NewError(fmt.Sprintf("failed to validate version with Edgegap API, status: %s", reply.Status), 13) // INTERNAL
	}

	// Store the Edgegap version
	versionData := map[string]interface{}{
		"version":    request.Version,
		"updated_at": time.Now().Unix(),
	}

	versionDataBytes, err := json.Marshal(versionData)
	if err != nil {
		return "", runtime.NewError("failed to marshal version data", 13) // INTERNAL
	}

	if _, err := nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      StorageCollectionEdgegapVersion,
			Key:             StorageKeyEdgegapVersion,
			Value:           string(versionDataBytes),
			PermissionRead:  2, // Public read
			PermissionWrite: 0, // No write from clients
		},
	}); err != nil {
		logger.Error("Failed to store Edgegap version: %v", err)
		return "", runtime.NewError("failed to store version", 13) // INTERNAL
	}

	logger.Info("Edgegap version updated to: %s", request.Version)

	// Return success response
	response := map[string]interface{}{
		"success": true,
		"version": request.Version,
		"message": "Edgegap version updated successfully. Will be used for new deployments immediately.",
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		return "", runtime.NewError("failed to marshal response", 13) // INTERNAL
	}

	return string(responseBytes), nil
}

// getEdgegapVersion retrieves the current Edgegap version configuration (S2S only)
func getEdgegapVersion(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	// This RPC can be called by servers with HTTP key authentication
	// If we reach this point with a user ID, it means a client is trying to call this RPC
	if _, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string); ok {
		logger.Warn("Client attempted to call server-to-server RPC for getting Edgegap version")
		return "", runtime.NewError("unauthorized: this RPC requires server authentication", 7) // PERMISSION_DENIED
	}

	// Check configuration to determine versioning mode
	env, ok := ctx.Value(runtime.RUNTIME_CTX_ENV).(map[string]string)
	if !ok {
		return "", runtime.NewError("failed to get environment context", 13) // INTERNAL
	}

	dynamicVersioning := false
	if dynamicVersioningStr, ok := env["EDGEGAP_DYNAMIC_VERSIONING"]; ok {
		var err error
		dynamicVersioning, err = strconv.ParseBool(dynamicVersioningStr)
		if err != nil {
			// Default to false if parsing fails
			dynamicVersioning = false
		}
	}

	response := map[string]interface{}{
		"dynamic_versioning": dynamicVersioning,
	}

	if dynamicVersioning {
		// Try to read version from storage
		objects, err := nk.StorageRead(ctx, []*runtime.StorageRead{
			{
				Collection: StorageCollectionEdgegapVersion,
				Key:        StorageKeyEdgegapVersion,
			},
		})
		
		if err != nil {
			logger.Error("Failed to read Edgegap version from storage: %v", err)
			return "", runtime.NewError(fmt.Sprintf("failed to read Edgegap version: %v", err), 13) // INTERNAL
		} else if len(objects) == 0 {
			// No version set yet
			response["error"] = "No Edgegap version configured"
			response["message"] = "Please set version using update_edgegap_version RPC"
		} else {
			// Parse stored version
			var storedData map[string]interface{}
			if err := json.Unmarshal([]byte(objects[0].Value), &storedData); err != nil {
				logger.Error("Failed to parse stored Edgegap version: %v", err)
				return "", runtime.NewError(fmt.Sprintf("failed to parse stored version: %v", err), 13) // INTERNAL
			}
			
			version, ok := storedData["version"].(string)
			if !ok || version == "" {
				return "", runtime.NewError("invalid Edgegap version format in storage", 13) // INTERNAL
			}
			
			response["version"] = version
			response["source"] = "dynamic"
			
			// Also get the updated_at timestamp if available
			if updatedAt, ok := storedData["updated_at"].(float64); ok {
				response["updated_at"] = int64(updatedAt)
			}
		}
	} else {
		// Static mode - return environment variable
		if version, ok := env["EDGEGAP_VERSION"]; ok {
			response["version"] = version
			response["source"] = "static"
		} else {
			response["error"] = "EDGEGAP_VERSION environment variable not set"
		}
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		return "", runtime.NewError("failed to marshal response", 13) // INTERNAL
	}

	return string(responseBytes), nil
}