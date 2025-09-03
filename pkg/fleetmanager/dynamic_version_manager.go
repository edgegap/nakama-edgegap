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
)

const (
	RpcIdUpdateEdgegapVersion = "update_edgegap_version"
	RpcIdGetEdgegapVersion    = "get_edgegap_version"
	
	// Error messages
	ErrorMessageUnauthorized = "unauthorized: this RPC requires server authentication"
	ErrorMessageNoVersionConfigured = "No Edgegap version configured"
	ErrorMessageSetVersionRPC = "Please set version using update_edgegap_version RPC"
	
	// Log messages
	LogMessageStoringInitialVersion = "No version found in storage, storing initial version: %s"
	LogMessageFailedStoreInitial = "Failed to store initial version during startup: %v"
	LogMessageFailedCheckVersion = "Failed to check for existing version during startup: %v"
	LogMessageVersionUpdated = "Edgegap version updated to: %s"
	LogMessageClientAttemptedS2S = "Client attempted to call server-to-server RPC"
	
	// Response fields
	ResponseFieldSource = "source"
	ResponseSourceDynamic = "dynamic"
)

type UpdateEdgegapVersionRequest struct {
	Version string `json:"version"`
}

// DynamicVersionManager manages dynamic versioning for Edgegap deployments
type DynamicVersionManager struct {
	config *EdgegapManagerConfiguration
	sm     *StorageManager
	logger runtime.Logger
}

// NewDynamicVersionManager creates a new DynamicVersionManager instance
func NewDynamicVersionManager(config *EdgegapManagerConfiguration, sm *StorageManager, logger runtime.Logger) *DynamicVersionManager {
	dvm := &DynamicVersionManager{
		config: config,
		sm:     sm,
		logger: logger,
	}
	
	// Check if initial version should be stored at startup
	if config.InitialVersion != "" {
		ctx := context.Background()
		// Check if a version is already stored
		_, _, err := sm.ReadEdgegapVersion(ctx)
		if err != nil {
			if errors.Is(err, ErrorNoVersionFound) {
				// No version in storage, store the initial version
				logger.Info(LogMessageStoringInitialVersion, config.InitialVersion)
				if err := sm.WriteEdgegapVersion(ctx, config.InitialVersion); err != nil {
					logger.Warn(LogMessageFailedStoreInitial, err)
				}
			} else {
				logger.Warn(LogMessageFailedCheckVersion, err)
			}
		}
	}
	
	return dvm
}

// ValidateVersionWithEdgegap validates that a version exists in Edgegap
func (dvm *DynamicVersionManager) ValidateVersionWithEdgegap(version string) error {
	apiHelper := helpers.NewAPIClient(dvm.config.ApiUrl, dvm.config.ApiToken)
	reply, err := apiHelper.Get(fmt.Sprintf("/v1/app/%s/version/%s", dvm.config.Application, version))
	if err != nil {
		return fmt.Errorf("failed to validate version with Edgegap API: %w", err)
	}
	
	if reply.StatusCode != http.StatusOK {
		if reply.StatusCode == http.StatusNotFound {
			return runtime.NewError(fmt.Sprintf("version '%s' does not exist for application '%s'", version, dvm.config.Application), 5) // NOT_FOUND
		}
		return runtime.NewError(fmt.Sprintf("failed to validate version with Edgegap API, status: %s", reply.Status), 13) // INTERNAL
	}
	
	return nil
}

// UpdateEdgegapVersion updates the Edgegap deployment version in storage (S2S only)
// Error codes used map to HTTP status codes via Nakama:
// - 3 (INVALID_ARGUMENT) → 400 Bad Request
// - 5 (NOT_FOUND) → 404 Not Found
// - 7 (PERMISSION_DENIED) → 403 Forbidden
// - 9 (FAILED_PRECONDITION) → 400 Bad Request
// - 13 (INTERNAL) → 500 Internal Server Error
func (dvm *DynamicVersionManager) UpdateEdgegapVersion(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	// This RPC should only be called by servers with HTTP key authentication, not by game clients
	// Nakama automatically validates the HTTP key when the Authorization header is provided
	// If we reach this point with a user ID, it means a client is trying to call this RPC
	if _, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string); ok {
		logger.Warn(LogMessageClientAttemptedS2S + " for Edgegap version update")
		return "", runtime.NewError(ErrorMessageUnauthorized, 7) // PERMISSION_DENIED
	}

	request := &UpdateEdgegapVersionRequest{}
	if err := json.Unmarshal([]byte(payload), request); err != nil {
		return "", runtime.NewError("invalid payload format", 3) // INVALID_ARGUMENT
	}

	// Validate version is not empty
	if request.Version == "" {
		return "", runtime.NewError("version cannot be empty", 3) // INVALID_ARGUMENT
	}


	// Validate the version exists in Edgegap before storing
	if err := dvm.ValidateVersionWithEdgegap(request.Version); err != nil {
		logger.Error("Failed to validate version with Edgegap: %v", err)
		return "", err
	}

	// Store the Edgegap version using StorageManager
	if err := dvm.sm.WriteEdgegapVersion(ctx, request.Version); err != nil {
		logger.Error("Failed to store Edgegap version: %v", err)
		return "", runtime.NewError("failed to store version", 13) // INTERNAL
	}

	logger.Info(LogMessageVersionUpdated, request.Version)

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

// GetEdgegapVersion retrieves the current Edgegap version configuration (S2S only)
func (dvm *DynamicVersionManager) GetEdgegapVersion(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	// This RPC can be called by servers with HTTP key authentication
	// If we reach this point with a user ID, it means a client is trying to call this RPC
	if _, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string); ok {
		logger.Warn(LogMessageClientAttemptedS2S + " for getting Edgegap version")
		return "", runtime.NewError(ErrorMessageUnauthorized, 7) // PERMISSION_DENIED
	}

	response := map[string]interface{}{}

	// Try to read version from storage using StorageManager
	version, updatedAt, err := dvm.sm.ReadEdgegapVersion(ctx)
	if err != nil {
		if errors.Is(err, ErrorNoVersionFound) {
			// No version set yet
			response["error"] = ErrorMessageNoVersionConfigured
			response["message"] = ErrorMessageSetVersionRPC
		} else {
			logger.Error("Failed to read Edgegap version from storage: %v", err)
			return "", runtime.NewError(fmt.Sprintf("failed to read Edgegap version: %v", err), 13) // INTERNAL
		}
	} else {
		response["version"] = version
		response[ResponseFieldSource] = ResponseSourceDynamic
		if updatedAt > 0 {
			response["updated_at"] = updatedAt
		}
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		return "", runtime.NewError("failed to marshal response", 13) // INTERNAL
	}

	return string(responseBytes), nil
}