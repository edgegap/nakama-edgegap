package fleetmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/edgegap/nakama-edgegap/internal/helpers"
	"github.com/heroiclabs/nakama-common/runtime"
	"sync"
)

var (
	fmInstance *EdgegapFleetManager
	once       sync.Once
)

// EdgegapFleetManager handles fleet management operations, interacting with the database,
// Nakama runtime, and Edgegap API.
type EdgegapFleetManager struct {
	ctx             context.Context
	logger          runtime.Logger
	nk              runtime.NakamaModule
	db              *sql.DB
	callbackHandler runtime.FmCallbackHandler
	edgegapManager  *EdgegapManager
	storageManager  *StorageManager
}

// NewEdgegapFleetManager initializes a new fleet manager instance with dependencies.
func NewEdgegapFleetManager(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, initializer runtime.Initializer) (*EdgegapFleetManager, error) {
	// Initialize Storage Manager
	sm := &StorageManager{
		nk:     nk,
		logger: logger,
	}

	// Initialize Edgegap Manager
	em, err := NewEdgegapManager(ctx, logger, initializer, sm)
	if err != nil {
		return nil, err
	}

	// Register Storage Index for tracking Edgegap instances
	if err := initializer.RegisterStorageIndex(
		StorageEdgegapIndex,
		StorageEdgegapInstancesCollection,
		"",
		[]string{"id", "create_time", "status", "player_count", "metadata"},
		[]string{"create_time", "player_count"},
		1_000_000,
		false,
	); err != nil {
		return nil, err
	}

	return &EdgegapFleetManager{
		ctx:             ctx,
		logger:          logger,
		nk:              nk,
		db:              db,
		callbackHandler: nil,
		edgegapManager:  em,
		storageManager:  sm,
	}, nil
}

// Init sets up the Nakama module and callback handler for the fleet manager.
func (efm *EdgegapFleetManager) Init(nk runtime.NakamaModule, callbackHandler runtime.FmCallbackHandler) error {
	efm.nk = nk
	efm.callbackHandler = callbackHandler

	once.Do(func() {
		fmInstance = efm
	})

	// Background worker to sync deployment info from Edgegap.
	// go fm.syncInstancesWorker()

	return nil
}

// Create provisions a new Edgegap deployment based on the given players.
func (efm *EdgegapFleetManager) Create(ctx context.Context, maxPlayers int, userIds []string, latencies []runtime.FleetUserLatencies, metadata map[string]any, callback runtime.FmCreateCallbackFn) error {
	efm.logger.Info("Requesting a new Deployment")
	callbackId := efm.callbackHandler.GenerateCallbackId()
	efm.callbackHandler.SetCallback(callbackId, callback)

	// Fetch IP addresses of users
	userIps, err := efm.storageManager.getUserIPs(ctx, userIds)
	if err != nil {
		efm.callbackHandler.InvokeCallback(callbackId, runtime.CreateError, nil, nil, nil, errors.New("unexpected Error while parsing Users Data"))
		return err
	}

	// Use caller IP if user IPs are unavailable
	if len(userIps) == 0 {
		callerIP, ok := ctx.Value(runtime.RUNTIME_CTX_CLIENT_IP).(string)
		if !ok {
			return ErrInvalidInput
		}
		userIps = append(userIps, callerIP)
	}

	// Request Edgegap deployment
	deployment, err := efm.edgegapManager.CreateDeployment(userIps, metadata)
	if err != nil {
		efm.logger.WithField("error", err).Error("failed to create Edgegap instance")
		efm.callbackHandler.InvokeCallback(callbackId, runtime.CreateError, nil, nil, nil, errors.New("error while communicating with Edgegap"))
		return err
	}

	// Validate Edgegap response
	if deployment.RequestId == "" {
		efm.logger.WithField("error", deployment.Message).Error("Failed to create Edgegap instance: %s", deployment.Message)
		efm.callbackHandler.InvokeCallback(callbackId, runtime.CreateError, nil, nil, nil, errors.New("error while creating Edgegap Deployment"))
		return errors.New("failed to create deployment")
	}

	// Store the new game session in the database
	_, err = efm.storageManager.createDbGameSession(ctx, deployment.RequestId, maxPlayers, userIds, callbackId, metadata)
	if err != nil {
		efm.logger.WithField("error", err).Error("failed to create Storage Game Session")
		efm.callbackHandler.InvokeCallback(callbackId, runtime.CreateError, nil, nil, nil, errors.New("error while creating Game Session"))
		return err
	}

	return nil
}

// Get retrieves a game session instance by its ID.
func (efm *EdgegapFleetManager) Get(ctx context.Context, id string) (*runtime.InstanceInfo, error) {
	return efm.storageManager.getDbGameSession(ctx, id)
}

// List retrieves game session instances based on a query, sorted by player count and creation time.
func (efm *EdgegapFleetManager) List(ctx context.Context, query string, limit int, cursor string) ([]*runtime.InstanceInfo, string, error) {
	entries, newCursor, err := efm.nk.StorageIndexList(ctx, "", StorageEdgegapIndex, query, limit, []string{"player_count", "-create_time"}, cursor)
	if err != nil {
		return nil, "", err
	}

	results := make([]*runtime.InstanceInfo, 0)
	for _, so := range entries.GetObjects() {
		var info *runtime.InstanceInfo
		if err = json.Unmarshal([]byte(so.Value), &info); err != nil {
			return nil, "", err
		}
		results = append(results, info)
	}

	return results, newCursor, nil
}

// Join allows users to join an existing game session.
func (efm *EdgegapFleetManager) Join(ctx context.Context, id string, userIds []string, metadata map[string]string) (*runtime.JoinInfo, error) {
	if id == "" {
		return nil, errors.New("expects id to be a valid GameSessionId")
	}

	instance, err := efm.storageManager.getDbGameSession(ctx, id)
	if err != nil {
		return nil, errors.New("instance not found")
	}

	if len(userIds) < 1 {
		return nil, errors.New("expects userIds to have at least one valid user id")
	}

	edgegapInstance, err := efm.storageManager.ExtractEdgegapInstance(instance)
	if err != nil {
		return nil, errors.New("error extracting Edgegap instance")
	}

	joinInfo := &runtime.JoinInfo{
		InstanceInfo: instance,
		SessionInfo:  nil,
	}

	// Unlimited player count (-1) allows immediate join
	if edgegapInstance.MaxPlayers < 0 {
		return joinInfo, nil
	}

	// Check if the session can accept more players
	if instance.PlayerCount+len(edgegapInstance.Reservations)+len(userIds) > edgegapInstance.MaxPlayers {
		return nil, errors.New("max players reservation limit reached")
	}

	// Add players to the reservation list
	for _, userId := range userIds {
		edgegapInstance.Reservations = helpers.AppendIfNotExists(edgegapInstance.Reservations, userId)
	}

	instance.Metadata["edgegap"] = edgegapInstance

	// Update the game session in the database
	err = efm.storageManager.updateDbGameSession(ctx, instance)
	if err != nil {
		return nil, errors.New("error updating db game session")
	}

	return joinInfo, nil
}

// Update modifies a game session's player count and metadata.
func (efm *EdgegapFleetManager) Update(ctx context.Context, id string, playerCount int, metadata map[string]any) error {
	instance, err := efm.storageManager.getDbGameSession(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to read instance info from db: %s", err.Error())
	}

	efm.logger.Warn("Player Count should not be updated manually and only from the Game Server SDK")
	instance.PlayerCount = playerCount

	return efm.storageManager.updateDbGameSession(ctx, instance)
}

// Delete removes a game session from the database.
func (efm *EdgegapFleetManager) Delete(ctx context.Context, id string) error {
	return efm.storageManager.deleteStorageGameSessions(ctx, []string{id})
}
