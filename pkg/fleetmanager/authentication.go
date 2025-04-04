package fleetmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

// OnAuthenticateUpdateDevice When the User connect with Device, update and fetch his Client IP, so it can be used to deploy Edgegap's Server
func OnAuthenticateUpdateDevice(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, in *api.AuthenticateDeviceRequest) error {
	return extractIPonAuth(ctx, logger, nk)
}

// OnAuthenticateUpdateCustom When the User connect with Custom, update and fetch his Client IP, so it can be used to deploy Edgegap's Server
func OnAuthenticateUpdateCustom(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, in *api.AuthenticateCustomRequest) error {
	return extractIPonAuth(ctx, logger, nk)
}

// OnAuthenticateUpdateApple When the User connect with Apple, update and fetch his Client IP, so it can be used to deploy Edgegap's Server
func OnAuthenticateUpdateApple(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, in *api.AuthenticateAppleRequest) error {
	return extractIPonAuth(ctx, logger, nk)
}

// OnAuthenticateUpdateEmail When the User connect with Email, update and fetch his Client IP, so it can be used to deploy Edgegap's Server
func OnAuthenticateUpdateEmail(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, in *api.AuthenticateEmailRequest) error {
	return extractIPonAuth(ctx, logger, nk)
}

// OnAuthenticateUpdateFacebook When the User connect with Facebook, update and fetch his Client IP, so it can be used to deploy Edgegap's Server
func OnAuthenticateUpdateFacebook(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, in *api.AuthenticateFacebookRequest) error {
	return extractIPonAuth(ctx, logger, nk)
}

// OnAuthenticateUpdateFacebookInstantInstance When the User connect with FacebookInstantInstance, update and fetch his Client IP, so it can be used to deploy Edgegap's Server
func OnAuthenticateUpdateFacebookInstantInstance(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, in *api.AuthenticateFacebookInstantGameRequest) error {
	return extractIPonAuth(ctx, logger, nk)
}

// OnAuthenticateUpdateSteam When the User connect with Steam, update and fetch his Client IP, so it can be used to deploy Edgegap's Server
func OnAuthenticateUpdateSteam(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, in *api.AuthenticateSteamRequest) error {
	return extractIPonAuth(ctx, logger, nk)
}

// OnAuthenticateUpdateInstanceCenter When the User connect with Instance Center, update and fetch his Client IP, so it can be used to deploy Edgegap's Server
func OnAuthenticateUpdateInstanceCenter(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, in *api.AuthenticateGameCenterRequest) error {
	return extractIPonAuth(ctx, logger, nk)
}

// OnAuthenticateUpdateGoogle When the User connect with Google, update and fetch his Client IP, so it can be used to deploy Edgegap's Server
func OnAuthenticateUpdateGoogle(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, out *api.Session, in *api.AuthenticateGoogleRequest) error {
	return extractIPonAuth(ctx, logger, nk)
}

func extractIPonAuth(ctx context.Context, logger runtime.Logger, nk runtime.NakamaModule) error {
	userIp := ctx.Value(runtime.RUNTIME_CTX_CLIENT_IP).(string)
	accountId := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	logger.Info("Update User %s IP: %s", accountId, userIp)

	account, err := nk.AccountGetId(ctx, accountId)
	if err != nil {
		return err
	}
	user := account.GetUser()
	var metadata map[string]any

	if err := json.Unmarshal([]byte(user.Metadata), &metadata); err != nil {
		return err
	}
	metadata["PlayerIp"] = userIp

	err = nk.AccountUpdateId(
		ctx,
		accountId,
		"",
		metadata,
		"",
		"",
		"",
		"",
		"",
	)

	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to update User %s", accountId)
	}

	return nil
}
