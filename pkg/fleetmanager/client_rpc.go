package fleetmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	RpcIdGameSessionList   = "game_list"
	RpcIdGameSessionGet    = "game_get"
	RpcIdGameSessionCreate = "game_create"
	RpcIdGameSessionJoin   = "game_join"
)

const (
	notificationConnectionInfo = 111
	notificationCreateTimeout  = 112
	notificationCreateFailed   = 113
)

type findGameSessionRequest struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}

type joinGameSessionRequest struct {
	GameID  string   `json:"game_id"`
	UserIds []string `json:"user_ids"`
}

type getGameSessionRequest struct {
	GameID string `json:"game_id"`
}

type createGameSessionRequest struct {
	UserIds    []string       `json:"user_ids"`
	MaxPlayers int            `json:"max_players"`
	Metadata   map[string]any `json:"metadata"`
}

type gameSessionListReply struct {
	Instances []*runtime.InstanceInfo `json:"instances"`
	Cursor    string                  `json:"cursor"`
}

type gameCreateReply struct {
	Message string `json:"message"`
	Ok      bool   `json:"ok"`
}

func createGameSession(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	userId, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok {
		return "", ErrInvalidInput
	}

	var req *createGameSessionRequest
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
				"IpAddress": instanceInfo.ConnectionInfo.IpAddress,
				"DnsName":   instanceInfo.ConnectionInfo.DnsName,
				"Port":      instanceInfo.ConnectionInfo.Port,
				"RequestId": instanceInfo.Id,
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

			// Send notification to client that game session creation timed out
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

			// Send notification to client that game session couldn't be created
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

	reply := gameCreateReply{
		Message: "Game Created",
		Ok:      true,
	}

	replyString, err := json.Marshal(reply)
	if err != nil {
		logger.WithField("error", err.Error()).Error("failed to marshal game create reply")
		return "", ErrInternalError
	}

	return string(replyString), err
}

func getGameSession(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	var req *getGameSessionRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		logger.WithField("error", err.Error()).Error("failed to unmarshal get Request")
		return "", ErrInternalError
	}

	efm := nk.GetFleetManager()
	instance, err := efm.Get(ctx, req.GameID)
	if err != nil {
		return "", err
	}

	replyString, err := json.Marshal(instance)
	if err != nil {
		logger.WithField("error", err.Error()).Error("failed to marshal game instance")
		return "", ErrInternalError
	}

	return string(replyString), nil
}

func joinGameSession(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	userId, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok {
		return "", ErrInvalidInput
	}

	var req *joinGameSessionRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		logger.WithField("error", err.Error()).Error("failed to unmarshal join Request")
		return "", ErrInternalError
	}

	if len(req.UserIds) == 0 {
		req.UserIds = []string{userId}
	}

	efm := nk.GetFleetManager()
	joinInfo, err := efm.Join(ctx, req.GameID, req.UserIds, nil)
	if err != nil {
		return "", err
	}

	replyString, err := json.Marshal(joinInfo)
	if err != nil {
		logger.WithField("error", err.Error()).Error("failed to marshal game instance")
		return "", ErrInternalError
	}

	return string(replyString), nil
}

func listGameSession(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	// value.metadata.edgegap.available_seats:>0
	var req *findGameSessionRequest
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &req); err != nil {
			logger.WithField("error", err.Error()).Error("failed to unmarshal list game request")
			return "", ErrInternalError
		}
	} else {
		req = &findGameSessionRequest{
			Limit: 10,
		}
	}

	efm := nk.GetFleetManager()
	instances, cursor, err := efm.List(ctx, req.Query, req.Limit, req.Cursor)
	if err != nil {
		logger.WithField("error", err.Error()).Error("failed to list game instances")
		return "", ErrInternalError
	}

	reply := &gameSessionListReply{
		Cursor:    cursor,
		Instances: instances,
	}
	replyString, err := json.Marshal(reply)
	if err != nil {
		logger.WithField("error", err.Error()).Error("failed to marshal game instances")
		return "", ErrInternalError
	}

	return string(replyString), nil
}
