package fleetmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/edgegap/nakama-edgegap/internal/helpers"
	"github.com/heroiclabs/nakama-common/runtime"
	"io"
	"net/http"
)

type EdgegapManager struct {
	configuration *EdgegapManagerConfiguration
	apiHelper     *helpers.APIClient
	logger        runtime.Logger
}

// NewEdgegapManager initializes a new EdgegapManager instance.
// It retrieves the configuration from environment variables, sets up API access,
// and registers necessary RPC functions.
func NewEdgegapManager(ctx context.Context, logger runtime.Logger, initializer runtime.Initializer, sm *StorageManager) (*EdgegapManager, error) {
	// Get the Configuration from Environment Variables
	configuration, err := NewEdgegapManagerConfiguration(ctx)
	if err != nil {
		return nil, err
	}

	// Retrieve Nakama configuration
	config, err := initializer.GetConfig()
	if err != nil {
		return nil, err
	}
	configuration.NakamaHttpKey = config.GetRuntime().GetHTTPKey()

	eem := &EdgegapEventManager{
		config: configuration,
		sm:     sm,
	}

	// Register RPC functions for handling various events
	rpcToRegisters := map[string]func(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error){
		RpcIdEventDeployment:   eem.handleDeploymentEvent,
		RpcIdEventConnection:   eem.handleConnectionEvent,
		RpcIdEventGame:         eem.handleGameEvent,
		RpcIdGameSessionCreate: createGameSession,
		RpcIdGameSessionGet:    getGameSession,
		RpcIdGameSessionJoin:   joinGameSession,
		RpcIdGameSessionList:   listGameSession,
	}

	// Register each RPC function with the Nakama runtime
	for rpcId, function := range rpcToRegisters {
		err = initializer.RegisterRpc(rpcId, function)
		if err != nil {
			return nil, err
		}
	}

	return &EdgegapManager{
		configuration: configuration,
		apiHelper:     helpers.NewAPIClient(configuration.ApiUrl, configuration.ApiToken),
		logger:        logger,
	}, nil
}

// getFormattedUrl constructs a formatted URL for Nakama API callbacks.
func (em *EdgegapManager) getFormattedUrl(path string) string {
	return fmt.Sprintf("%s/v2/rpc/%s?http_key=%s&unwrap", em.configuration.NakamaAccessUrl, path, em.configuration.NakamaHttpKey)
}

// CreateDeployment initiates a new deployment on Edgegap using the given users' IP addresses and metadata.
func (em *EdgegapManager) CreateDeployment(usersIP []string, metadata map[string]any) (*EdgegapBetaDeployment, error) {
	// Prepare deployment data
	deployment, err := em.getDeploymentCreation(usersIP, metadata)
	if err != nil {
		return nil, err
	}

	// Send deployment request to Edgegap API
	reply, err := em.apiHelper.Post("/beta/deployments", deployment)
	if err != nil {
		return nil, err
	}
	defer reply.Body.Close()

	// Check if request was accepted
	if reply.StatusCode != http.StatusAccepted {
		body, err := io.ReadAll(reply.Body)
		if err != nil {
			return nil, err
		}
		var msg EdgegapBetaDeployment
		err = json.Unmarshal(body, &msg)
		if err != nil {
			return nil, err
		}
		return &msg, errors.New("could not create deployment")
	}

	// Parse the response body
	body, err := io.ReadAll(reply.Body)
	if err != nil {
		return nil, err
	}

	var response EdgegapBetaDeployment
	err = json.Unmarshal(body, &response)

	return &response, err
}

// getDeploymentCreation prepares the deployment payload, including metadata and environment variables.
func (em *EdgegapManager) getDeploymentCreation(usersIP []string, metadata map[string]any) (*EdgegapDeploymentCreation, error) {
	var users []EdgegapDeploymentUser

	// Convert user IPs into EdgegapDeploymentUser objects
	for _, ip := range usersIP {
		users = append(users, EdgegapDeploymentUser{
			IpAddress: ip,
		})
	}

	// Marshal metadata into JSON format
	metadataValue, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	// Construct deployment request payload
	return &EdgegapDeploymentCreation{
		ApplicationName: em.configuration.Application,
		Version:         em.configuration.Version,
		Users:           users,
		EnvironmentVariables: []EdgegapEnvironmentVariable{
			{
				Key:      "NAKAMA_CONNECTION_EVENT_URL",
				Value:    em.getFormattedUrl(RpcIdEventConnection),
				IsHidden: true,
			},
			{
				Key:      "NAKAMA_GAME_EVENT_URL",
				Value:    em.getFormattedUrl(RpcIdEventGame),
				IsHidden: true,
			},
			{
				Key:      "NAKAMA_GAME_METADATA",
				Value:    string(metadataValue),
				IsHidden: false,
			},
		},
		Tags: []string{
			"nakama",
		},
		Webhook: EdgegapWebhook{
			Url: em.getFormattedUrl(RpcIdEventDeployment),
		},
	}, nil
}

// StopDeployment sends a request to stop an active deployment on Edgegap.
func (em *EdgegapManager) StopDeployment(requestID string) (*EdgegapApiMessage, error) {
	// Send stop request to Edgegap API
	reply, err := em.apiHelper.Delete("/v1/stop/" + requestID)
	if err != nil {
		return nil, err
	}
	defer reply.Body.Close()

	// Check if request was successful
	if reply.StatusCode == http.StatusOK || reply.StatusCode == http.StatusAccepted {
		body, err := io.ReadAll(reply.Body)
		if err != nil {
			return nil, err
		}
		var message EdgegapApiMessage
		err = json.Unmarshal(body, &message)

		return &message, err
	}

	return nil, errors.New("Error stopping edgegap deployment " + requestID)
}
