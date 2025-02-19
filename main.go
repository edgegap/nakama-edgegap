package main

import (
	"context"
	"database/sql"
	"github.com/edgegap/nakama-edgegap/pkg/fleetmanager"
	"github.com/heroiclabs/nakama-common/runtime"
	"time"
)

func InitModule(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, initializer runtime.Initializer) error {
	initStart := time.Now()

	// Register the Fleet Manager
	efm, err := fleetmanager.NewEdgegapFleetManager(ctx, logger, db, nk, initializer)
	if err != nil {
		return err
	}

	if err = initializer.RegisterFleetManager(efm); err != nil {
		logger.WithField("error", err).Error("failed to register Edgegap fleet manager")
		return err
	}

	// Register Authentication Methods
	if err := initializer.RegisterAfterAuthenticateCustom(fleetmanager.OnAuthenticateUpdateCustom); err != nil {
		return err
	}

	logger.Info("Edgegap Plugin loaded in '%s'", time.Now().Sub(initStart).String())

	return nil
}
