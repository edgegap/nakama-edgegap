package fleetmanager

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/heroiclabs/nakama-common/runtime"
	"time"
)

const (
	// Constants for storage collection and index names
	StorageEdgegapIndex               = "_edgegap_instances_idx"
	StorageEdgegapInstancesCollection = "_edgegap_instances"
)

const (
	// Constants representing different statuses of an Edgegap instance
	EdgegapStatusRequested = "REQUESTED"
	EdgegapStatusRunning   = "RUNNING"
	EdgegapStatusReady     = "READY"
	EdgegapStatusStopping  = "STOPPING"
	EdgegapStatusError     = "ERROR"
	EdgegapStatusUnknown   = "UNKNOWN"
)

// StorageManager handles interactions with Nakama's storage system
type StorageManager struct {
	nk     runtime.NakamaModule
	logger runtime.Logger
}

// updateDbInstanceSession updates an existing instance session in the database.
func (sm *StorageManager) updateDbInstanceSession(ctx context.Context, instance *runtime.InstanceInfo) error {
	// Sync instance metadata before updating storage
	err := sm.SyncInstance(instance)
	if err != nil {
		return err
	}

	// Serialize instance data to JSON
	value, err := json.Marshal(instance)
	if err != nil {
		return err
	}

	// Write updated instance to storage
	sw := runtime.StorageWrite{
		Collection: StorageEdgegapInstancesCollection,
		Key:        instance.Id,
		UserID:     "",
		Value:      string(value),
	}
	_, err = sm.nk.StorageWrite(ctx, []*runtime.StorageWrite{&sw})
	return err
}

// ExtractEdgegapInstance extracts Edgegap-related data from an instance's metadata.
func (sm *StorageManager) ExtractEdgegapInstance(instance *runtime.InstanceInfo) (*EdgegapInstanceInfo, error) {
	// Check if metadata contains "edgegap" key
	value, ok := instance.Metadata["edgegap"]
	if !ok {
		return nil, errors.New("edgegap key not in metadata")
	}

	// Convert metadata to JSON and unmarshal into EdgegapInstanceInfo struct
	valueString, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var edgegapInstance EdgegapInstanceInfo
	if err := json.Unmarshal(valueString, &edgegapInstance); err != nil {
		return nil, err
	}

	return &edgegapInstance, nil
}

// SyncInstance synchronizes Edgegap instance metadata, including player count and available seats.
func (sm *StorageManager) SyncInstance(instance *runtime.InstanceInfo) error {
	// Extract Edgegap instance information
	edgegapInstance, err := sm.ExtractEdgegapInstance(instance)
	if err != nil {
		sm.logger.Error("Error extracting edgegap instance: %v", err)
		return err
	}

	// Get available seats in the session
	availableSeat, err := sm.GetAvailableSeat(instance)
	if err != nil {
		return err
	}

	// Update player count and available seats
	instance.PlayerCount = len(edgegapInstance.Connections)
	edgegapInstance.AvailableSeats = availableSeat

	// Save updated metadata back into the instance
	instance.Metadata["edgegap"] = edgegapInstance

	return nil
}

// GetAvailableSeat calculates the number of available seats in an instance.
func (sm *StorageManager) GetAvailableSeat(instance *runtime.InstanceInfo) (int, error) {
	edgegapInstance, err := sm.ExtractEdgegapInstance(instance)
	if err != nil {
		return 0, err
	}

	// Calculate available seats based on max players and reservations
	if edgegapInstance.MaxPlayers > 0 {
		return edgegapInstance.MaxPlayers - len(edgegapInstance.Reservations) - len(edgegapInstance.Connections), nil
	}

	// Return -1 if maxPlayers is not set
	return -1, nil
}

// createDbInstanceSession creates and stores a new instance session in the database.
func (sm *StorageManager) createDbInstanceSession(ctx context.Context, id string, maxPlayers int, userIds []string, callbackId string, metadata map[string]any) (*runtime.InstanceInfo, error) {
	// Initialize metadata if nil
	if metadata == nil {
		metadata = make(map[string]any)
	}

	// Store Edgegap-related information in metadata
	metadata["edgegap"] = EdgegapInstanceInfo{
		MaxPlayers:   maxPlayers,
		Reservations: userIds,
		CallbackId:   callbackId,
		Connections:  []string{},
	}

	// Create a new instance session instance
	instance := &runtime.InstanceInfo{
		Id:             id,
		ConnectionInfo: nil,
		CreateTime:     time.Now(),
		PlayerCount:    0,
		Status:         EdgegapStatusRequested,
		Metadata:       metadata,
	}

	// Synchronize instance before storing
	err := sm.SyncInstance(instance)
	if err != nil {
		return nil, err
	}

	// Serialize instance to JSON and store in Nakama
	value, err := json.Marshal(instance)
	if err != nil {
		return nil, err
	}

	sw := runtime.StorageWrite{
		Collection: StorageEdgegapInstancesCollection,
		Key:        id,
		UserID:     "",
		Value:      string(value),
	}

	_, err = sm.nk.StorageWrite(ctx, []*runtime.StorageWrite{&sw})
	return instance, err
}

// listDbInstanceSessions retrieves all stored instance sessions from Nakama.
func (sm *StorageManager) listDbInstanceSessions(ctx context.Context) ([]*runtime.InstanceInfo, error) {
	instances := make([]*runtime.InstanceInfo, 0)
	cursor := ""

	// Loop to fetch sessions in batches
	for {
		objects, nextCursor, err := sm.nk.StorageList(ctx, "", "", StorageEdgegapInstancesCollection, 1_000, cursor)
		if err != nil {
			return nil, err
		}

		// Deserialize each stored object into an instance
		for _, obj := range objects {
			var info *runtime.InstanceInfo
			if err = json.Unmarshal([]byte(obj.Value), &info); err != nil {
				return nil, err
			}
			instances = append(instances, info)
		}

		// Stop if no more results
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return instances, nil
}

// getDbInstanceSession retrieves a single instance session by ID from the Nakama database.
func (sm *StorageManager) getDbInstanceSession(ctx context.Context, id string) (*runtime.InstanceInfo, error) {
	objects, err := sm.nk.StorageRead(ctx, []*runtime.StorageRead{{
		Collection: StorageEdgegapInstancesCollection,
		Key:        id,
	}})
	if err != nil {
		return nil, err
	}

	// If no session is found, return nil
	if len(objects) == 0 {
		return nil, nil
	}

	obj := objects[0]

	// Deserialize stored JSON into an instance
	var instance *runtime.InstanceInfo
	if err = json.Unmarshal([]byte(obj.Value), &instance); err != nil {
		return nil, err
	}

	return instance, nil
}

// deleteStorageInstanceSessions removes instance sessions from Nakama storage.
func (sm *StorageManager) deleteStorageInstanceSessions(ctx context.Context, ids []string) error {
	deletes := make([]*runtime.StorageDelete, 0, len(ids))

	// Prepare delete requests for each session ID
	for _, id := range ids {
		deletes = append(deletes, &runtime.StorageDelete{
			Collection: StorageEdgegapInstancesCollection,
			Key:        id,
		})
	}

	// Execute delete operation
	if err := sm.nk.StorageDelete(ctx, deletes); err != nil {
		return err
	}

	return nil
}

// getUserIPs retrieves player IP addresses from their metadata.
func (sm *StorageManager) getUserIPs(ctx context.Context, userIds []string) ([]string, error) {
	userIps := make([]string, 0)

	// Iterate through user IDs and fetch their metadata
	for _, userId := range userIds {
		userAccount, err := sm.nk.AccountGetId(ctx, userId)
		if err != nil {
			return nil, err
		}

		// Parse user metadata from JSON
		userMetadata := make(map[string]interface{})
		err = json.Unmarshal([]byte(userAccount.User.Metadata), &userMetadata)
		if err != nil {
			return nil, err
		}

		// Extract IP address if available
		userIp, ok := userMetadata["PlayerIp"]
		if !ok {
			sm.logger.Warn("User %s metadata does not contain PlayerIp", userId)
			continue
		}
		if userIp != "" {
			userIps = append(userIps, userIp.(string))
		}
	}

	return userIps, nil
}
