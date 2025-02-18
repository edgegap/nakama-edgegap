package fleetmanager

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/heroiclabs/nakama-common/runtime"
	"time"
)

const (
	StorageEdgegapIndex               = "_edgegap_instances_idx"
	StorageEdgegapInstancesCollection = "_edgegap_instances"
)

const (
	EdgegapStatusRequested = "REQUESTED"
	EdgegapStatusRunning   = "RUNNING"
	EdgegapStatusReady     = "READY"
	EdgegapStatusStopping  = "STOPPING"
	EdgegapStatusError     = "ERROR"
	EdgegapStatusUnknown   = "UNKNOWN"
)

type StorageManager struct {
	nk     runtime.NakamaModule
	logger runtime.Logger
}

func (sm *StorageManager) updateDbGameSession(ctx context.Context, instance *runtime.InstanceInfo) error {
	err := sm.SyncInstance(instance)
	if err != nil {
		return err
	}

	value, err := json.Marshal(instance)
	if err != nil {
		return err
	}

	sw := runtime.StorageWrite{
		Collection: StorageEdgegapInstancesCollection,
		Key:        instance.Id,
		UserID:     "",
		Value:      string(value),
	}
	_, err = sm.nk.StorageWrite(ctx, []*runtime.StorageWrite{&sw})
	return err
}

func (sm *StorageManager) ExtractEdgegapInstance(instance *runtime.InstanceInfo) (*EdgegapInstanceInfo, error) {
	value, ok := instance.Metadata["edgegap"]
	if !ok {
		return nil, errors.New("edgegap key not in metadata")
	}
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

func (sm *StorageManager) SyncInstance(instance *runtime.InstanceInfo) error {
	edgegapInstance, err := sm.ExtractEdgegapInstance(instance)
	if err != nil {
		sm.logger.Error("Error extracting edgegap instance: %v", err)
		return err
	}

	availableSeat, err := sm.GetAvailableSeat(instance)
	if err != nil {
		return err
	}
	// Update the players Count to match the active connection and the available seat
	instance.PlayerCount = len(edgegapInstance.Connections)
	edgegapInstance.AvailableSeats = availableSeat

	instance.Metadata["edgegap"] = edgegapInstance

	return nil
}

func (sm *StorageManager) GetAvailableSeat(instance *runtime.InstanceInfo) (int, error) {
	edgegapInstance, err := sm.ExtractEdgegapInstance(instance)
	if err != nil {
		return 0, err
	}

	if edgegapInstance.MaxPlayers > 0 {
		return edgegapInstance.MaxPlayers - len(edgegapInstance.Reservations) - len(edgegapInstance.Connections), nil
	}

	return -1, nil
}

func (sm *StorageManager) createDbGameSession(ctx context.Context, id string, maxPlayers int, userIds []string, callbackId string, metadata map[string]any) (*runtime.InstanceInfo, error) {
	if metadata == nil {
		metadata = make(map[string]any)
	}

	metadata["edgegap"] = EdgegapInstanceInfo{
		MaxPlayers:   maxPlayers,
		Reservations: userIds,
		CallbackId:   callbackId,
		Connections:  []string{},
	}
	instance := &runtime.InstanceInfo{
		Id:             id,
		ConnectionInfo: nil,
		CreateTime:     time.Now(),
		PlayerCount:    0,
		Status:         EdgegapStatusRequested,
		Metadata:       metadata,
	}
	err := sm.SyncInstance(instance)
	if err != nil {
		return nil, err
	}

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

// Lists all Session stored in Nakama.
func (sm *StorageManager) listDbGameSessions(ctx context.Context) ([]*runtime.InstanceInfo, error) {
	instances := make([]*runtime.InstanceInfo, 0)

	cursor := ""
	for {
		objects, nextCursor, err := sm.nk.StorageList(ctx, "", "", StorageEdgegapInstancesCollection, 1_000, cursor)
		if err != nil {
			return nil, err
		}

		for _, obj := range objects {
			var info *runtime.InstanceInfo
			if err = json.Unmarshal([]byte(obj.Value), &info); err != nil {
				return nil, err
			}
			instances = append(instances, info)
		}

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return instances, nil
}

// Returns a Nakama DB GameSession.
func (sm *StorageManager) getDbGameSession(ctx context.Context, id string) (*runtime.InstanceInfo, error) {
	objects, err := sm.nk.StorageRead(ctx, []*runtime.StorageRead{{
		Collection: StorageEdgegapInstancesCollection,
		Key:        id,
	}})
	if err != nil {
		return nil, err
	}

	if len(objects) == 0 {
		return nil, nil
	}

	obj := objects[0]

	var instance *runtime.InstanceInfo
	if err = json.Unmarshal([]byte(obj.Value), &instance); err != nil {
		return nil, err
	}

	return instance, nil
}

// Delete Game Session from the Nakama DB. This also updates the associated storage index.
func (sm *StorageManager) deleteStorageGameSessions(ctx context.Context, ids []string) error {
	deletes := make([]*runtime.StorageDelete, 0, len(ids))
	for _, id := range ids {
		deletes = append(deletes, &runtime.StorageDelete{
			Collection: StorageEdgegapInstancesCollection,
			Key:        id,
		})
	}

	if err := sm.nk.StorageDelete(ctx, deletes); err != nil {
		return err
	}

	return nil
}

// GetUserIPs allow to extract the user IP stored in the metadata of the User
func (sm *StorageManager) getUserIPs(ctx context.Context, userIds []string) ([]string, error) {
	userIps := make([]string, 0)
	for _, userId := range userIds {
		userAccount, err := sm.nk.AccountGetId(ctx, userId)
		if err != nil {
			return nil, err
		}
		userMetadata := make(map[string]interface{})
		err = json.Unmarshal([]byte(userAccount.User.Metadata), &userMetadata)
		if err != nil {
			return nil, err
		}

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
