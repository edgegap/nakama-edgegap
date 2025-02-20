package fleetmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	RpcIdInstanceSessionList   = "instance_list"
	RpcIdInstanceSessionGet    = "instance_get"
	RpcIdInstanceSessionCreate = "instance_create"
	RpcIdInstanceSessionJoin   = "instance_join"
)

const (
	notificationConnectionInfo = 111
	notificationCreateTimeout  = 112
	notificationCreateFailed   = 113
)

type findInstanceSessionRequest struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}

type joinInstanceSessionRequest struct {
	InstanceID string   `json:"instance_id"`
	UserIds    []string `json:"user_ids"`
}

type getInstanceSessionRequest struct {
	InstanceID string `json:"instance_id"`
}

type createInstanceSessionRequest struct {
	UserIds    []string       `json:"user_ids"`
	MaxPlayers int            `json:"max_players"`
	Metadata   map[string]any `json:"metadata"`
}

type instanceSessionListReply struct {
	Instances []*runtime.InstanceInfo `json:"instances"`
	Cursor    string                  `json:"cursor"`
}

type instanceCreateReply struct {
	Message string `json:"message"`
	Ok      bool   `json:"ok"`
}

// createInstanceSession client rpc to create a instance
func createInstanceSession(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	userId, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok {
		return "", ErrInvalidInput
	}

	var req *createInstanceSessionRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		logger.WithField("error", err.Error()).Error("failed to unmarshal create Request")
		return "", ErrInternalError
	}

	if len(req.UserIds) == 0 {
		req.UserIds = []string{userId}
	}

	var callback runtime.FmCreateCallbackFn = func(status runtime.FmCreateStatus, instanceInfo *runtime.InstanceInfo, sessionInfo []*runtime.SessionInfo, metadata map[string]any, createErr error) {
		switch status {
		case runtime.CreateSuccess:
			logger.Info("Edgegap instance created: %s", instanceInfo.Id)

			content := map[string]interface{}{
				"IpAddress":  instanceInfo.ConnectionInfo.IpAddress,
				"DnsName":    instanceInfo.ConnectionInfo.DnsName,
				"Port":       instanceInfo.ConnectionInfo.Port,
				"InstanceId": instanceInfo.Id,
			}
			// Send connection details notifications to players
			for _, userId := range req.UserIds {
				subject := "connection-info"

				code := notificationConnectionInfo
				err := nk.NotificationSend(ctx, userId, subject, content, code, "", false)
				if err != nil {
					logger.WithField("error", err.Error()).Error("Failed to send notification")
				}
			}
			return
		case runtime.CreateTimeout:
			logger.WithField("error", createErr.Error()).Error("Failed to create Edgegap instance, timed out")

			// Send notification to client that instance session creation timed out
			for _, userId := range req.UserIds {
				subject := "create-timeout"
				content := map[string]interface{}{}
				code := notificationCreateTimeout
				err := nk.NotificationSend(ctx, userId, subject, content, code, "", false)
				if err != nil {
					logger.WithField("error", err.Error()).Error("Failed to send notification")
				}
			}
		default:
			logger.WithField("error", createErr.Error()).Error("Failed to create Edgegap instance")

			// Send notification to client that instance session couldn't be created
			for _, userId := range req.UserIds {
				subject := "create-failed"
				content := map[string]interface{}{}
				code := notificationCreateFailed
				err := nk.NotificationSend(ctx, userId, subject, content, code, "", false)
				if err != nil {
					logger.WithField("error", err.Error()).Error("Failed to send notification")
				}
			}
			return
		}
	}

	efm := nk.GetFleetManager()
	err := efm.Create(ctx, req.MaxPlayers, req.UserIds, nil, req.Metadata, callback)

	reply := instanceCreateReply{
		Message: "Instance Created",
		Ok:      true,
	}

	replyString, err := json.Marshal(reply)
	if err != nil {
		logger.WithField("error", err.Error()).Error("failed to marshal instance create reply")
		return "", ErrInternalError
	}

	return string(replyString), err
}

// getInstanceSession client rpc to retrieve the instance info of a instance
func getInstanceSession(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var req *getInstanceSessionRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		logger.WithField("error", err.Error()).Error("failed to unmarshal get Request")
		return "", ErrInternalError
	}

	efm := nk.GetFleetManager()
	instance, err := efm.Get(ctx, req.InstanceID)
	if err != nil {
		return "", err
	}

	replyString, err := json.Marshal(instance)
	if err != nil {
		logger.WithField("error", err.Error()).Error("failed to marshal instance instance")
		return "", ErrInternalError
	}

	return string(replyString), nil
}

// joinInstanceSession client rpc to join a instance with seats reservations
func joinInstanceSession(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	userId, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok {
		return "", ErrInvalidInput
	}

	var req *joinInstanceSessionRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		logger.WithField("error", err.Error()).Error("failed to unmarshal join Request")
		return "", ErrInternalError
	}

	if len(req.UserIds) == 0 {
		req.UserIds = []string{userId}
	}

	efm := nk.GetFleetManager()
	joinInfo, err := efm.Join(ctx, req.InstanceID, req.UserIds, nil)
	if err != nil {
		return "", err
	}

	replyString, err := json.Marshal(joinInfo)
	if err != nil {
		logger.WithField("error", err.Error()).Error("failed to marshal instance instance")
		return "", ErrInternalError
	}

	return string(replyString), nil
}

// listInstanceSession client rpc to list instances with query
// Example to list all ready instances with at least 1 available seat
// query="+value.metadata.edgegap.available_seats:>=1 +value.status:READY"
func listInstanceSession(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {

	var req *findInstanceSessionRequest
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &req); err != nil {
			logger.WithField("error", err.Error()).Error("failed to unmarshal list instance request")
			return "", ErrInternalError
		}
	} else {
		req = &findInstanceSessionRequest{
			Limit: 10,
		}
	}

	efm := nk.GetFleetManager()
	instances, cursor, err := efm.List(ctx, req.Query, req.Limit, req.Cursor)
	if err != nil {
		logger.WithField("error", err.Error()).Error("failed to list instance instances")
		return "", ErrInternalError
	}

	reply := &instanceSessionListReply{
		Cursor:    cursor,
		Instances: instances,
	}
	replyString, err := json.Marshal(reply)
	if err != nil {
		logger.WithField("error", err.Error()).Error("failed to marshal instance instances")
		return "", ErrInternalError
	}

	return string(replyString), nil
}
