package fleetmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/edgegap/nakama-edgegap/internal/helpers"
	"github.com/heroiclabs/nakama-common/runtime"
	"strings"
)

const (
	RpcIdEventDeployment = "edgegap_deployment"
	RpcIdEventConnection = "edgegap_connection"
	RpcIdEventGame       = "edgegap_game"
)

var (
	ErrInvalidInput  = runtime.NewError("input is invalid", 3)       // INVALID_ARGUMENT
	ErrInternalError = runtime.NewError("internal server error", 13) // INTERNAL
)

type EventMessage struct {
	payload string
	headers map[string][]string
	params  map[string][]string
}

type EdgegapEventManager struct {
	config *EdgegapManagerConfiguration
	sm     *StorageManager
}

// unpack extracts headers and query parameters from the context
// and returns an EventMessage struct containing them along with the payload.
func (eem *EdgegapEventManager) unpack(ctx context.Context, payload string) (*EventMessage, error) {
	headers, ok := ctx.Value(runtime.RUNTIME_CTX_HEADERS).(map[string][]string)
	if !ok {
		return nil, ErrInternalError
	}

	params, ok := ctx.Value(runtime.RUNTIME_CTX_QUERY_PARAMS).(map[string][]string)
	if !ok {
		return nil, ErrInternalError
	}

	return &EventMessage{
		payload: payload,
		headers: headers,
		params:  params,
	}, nil
}

// handleDeploymentEvent processes deployment-related events.
// It extracts the payload, updates the game session status, and logs errors if necessary.
func (eem *EdgegapEventManager) handleDeploymentEvent(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	logger.Info("Handle Deployment")
	msg, err := eem.unpack(ctx, payload)
	if err != nil {
		return "", err
	}

	var deployment EdgegapDeploymentStatus
	if err := json.Unmarshal([]byte(msg.payload), &deployment); err != nil {
		return "", err
	}

	instance, err := eem.sm.getDbGameSession(ctx, deployment.RequestId)
	if err != nil {
		return "", err
	}
	if instance == nil {
		return "", errors.New("no instance found with requestId " + deployment.RequestId)
	}

	badState := true

	switch deployment.CurrentStatus {
	case DeploymentStatusReady:
		logger.Info("Edgegap deployment ready #%s", deployment.RequestId)
		instance.Status = EdgegapStatusRunning
		instance.ConnectionInfo = &runtime.ConnectionInfo{
			IpAddress: deployment.PublicIp,
			DnsName:   deployment.Fqdn,
			Port:      deployment.Ports[eem.config.PortName].External,
		}
		badState = false
	case DeploymentStatusError:
		logger.Warn("Edgegap deployment error #%s : %s", deployment.RequestId, deployment.Error)
		instance.Status = EdgegapStatusError
	default:
		logger.Error("Unknown deployment status %s", deployment.CurrentStatus)
		instance.Status = EdgegapStatusUnknown
	}

	if badState {
		ei, err := eem.sm.ExtractEdgegapInstance(instance)
		if err != nil {
			return "", err
		}
		fmInstance.callbackHandler.InvokeCallback(ei.CallbackId, runtime.CreateError, nil, nil, nil, errors.New("an error occurred with edgegap deployment"))
	}

	err = eem.sm.updateDbGameSession(ctx, instance)
	if err != nil {
		return "", err
	}

	return "ok", nil
}

// handleConnectionEvent processes connection-related events.
// It updates the game session's connection and reservation metadata.
func (eem *EdgegapEventManager) handleConnectionEvent(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	msg, err := eem.unpack(ctx, payload)
	if err != nil {
		return "", err
	}

	var connectionEvent ConnectionEventMessage
	if err := json.Unmarshal([]byte(msg.payload), &connectionEvent); err != nil {
		return "", err
	}

	instance, err := eem.sm.getDbGameSession(ctx, connectionEvent.GameId)
	if err != nil {
		return "", err
	}

	if instance == nil {
		return "", errors.New("no instance found with gameId " + connectionEvent.GameId)
	}

	edgegapInstance, err := eem.sm.ExtractEdgegapInstance(instance)
	if err != nil {
		return "", err
	}

	// We want to move all reservations present in the Connections List
	newReservations := helpers.RemoveElements(edgegapInstance.Reservations, connectionEvent.Connections)
	edgegapInstance.Reservations = newReservations
	edgegapInstance.Connections = connectionEvent.Connections
	instance.Metadata["edgegap"] = edgegapInstance

	err = eem.sm.updateDbGameSession(ctx, instance)
	if err != nil {
		return "", err
	}

	return "ok", nil
}

// handleGameEvent processes game state change events.
// It updates the game session's status based on the event action.
func (eem *EdgegapEventManager) handleGameEvent(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	msg, err := eem.unpack(ctx, payload)
	if err != nil {
		return "", err
	}

	var gameEvent GameEventMessage
	if err := json.Unmarshal([]byte(msg.payload), &gameEvent); err != nil {
		return "", err
	}

	instance, err := eem.sm.getDbGameSession(ctx, gameEvent.GameId)
	if err != nil {
		return "", err
	}

	if instance == nil {
		return "", errors.New("no instance found with gameId " + gameEvent.GameId)
	}

	stopping := false

	switch strings.ToUpper(gameEvent.Action) {
	case GameEventStateReady:
		logger.Info("Edgegap game ready id=%s : %s", gameEvent.GameId, gameEvent.Message)
		instance.Status = EdgegapStatusReady

		// Extract new Metadata coming from the Game Server and merge it with current
		instance.Metadata = helpers.MergeMaps(instance.Metadata, gameEvent.Metadata)

		ei, err := eem.sm.ExtractEdgegapInstance(instance)
		if err != nil {
			return "", err
		}
		fmInstance.callbackHandler.InvokeCallback(ei.CallbackId, runtime.CreateSuccess, instance, nil, nil, nil)

	case GameEventStateStop:
		logger.Info("Edgegap game stop #%s: %s", gameEvent.GameId, gameEvent.Message)
		instance.Status = EdgegapStatusStopping
		stopping = true

	case GameEventStateError:
		logger.Error("Edgegap game state error #%s: %s", gameEvent.GameId, gameEvent.Message)
		instance.Status = EdgegapStatusError

	default:
		logger.Error("Unknown action #%s: %s", gameEvent.Action, gameEvent.Message)
		instance.Status = EdgegapStatusUnknown
	}

	err = eem.sm.updateDbGameSession(ctx, instance)
	if err != nil {
		return "", err
	}

	if stopping {
		_, err := fmInstance.edgegapManager.StopDeployment(gameEvent.GameId)
		if err != nil {
			return "", err
		}
	}

	return "ok", nil
}
