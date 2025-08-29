package fleetmanager

import (
	"context"
	"errors"
	"fmt"
	"github.com/edgegap/nakama-edgegap/internal/helpers"
	"github.com/heroiclabs/nakama-common/runtime"
	"net/http"
	"strings"
	"time"
)

type EdgegapManagerConfiguration struct {
	NakamaNode             string `json:"nakama_node"`
	ApiUrl                 string `json:"base_url"`
	ApiToken               string `json:"api_token"`
	Application            string `json:"application"`
	Version                string `json:"version"`
	DynamicVersioning      bool   `json:"dynamic_versioning"`
	PortName               string `json:"port_name"`
	NakamaAccessUrl        string `json:"nakama_access_url"`
	NakamaHttpKey          string `json:"nakama_http_key"`
	PollingInterval        string `json:"polling_interval"`
	CleanupInterval        string `json:"cleanup_interval"`
	ReservationMaxDuration string `json:"reservation_max_duration"`
}

// NewEdgegapManagerConfiguration Create New Edgegap EdgegapManager Configuration and Fail if missing values
func NewEdgegapManagerConfiguration(ctx context.Context) (*EdgegapManagerConfiguration, error) {
	nakamaNode, ok := ctx.Value(runtime.RUNTIME_CTX_NODE).(string)
	if !ok || !strings.HasPrefix(nakamaNode, "nakama") {
		return nil, errors.New("failed to get nakama node from ctx")
	}

	env, ok := ctx.Value(runtime.RUNTIME_CTX_ENV).(map[string]string)
	if !ok {
		return nil, runtime.NewError("expects env ctx value to be a map[string]string", 3)
	}

	url, ok := env["EDGEGAP_API_URL"]
	if !ok {
		return nil, runtime.NewError("EDGEGAP_API_URL not found in environment", 3)
	}
	token, ok := env["EDGEGAP_API_TOKEN"]
	if !ok {
		return nil, runtime.NewError("EDGEGAP_API_TOKEN not found in environment", 3)
	}

	app, ok := env["EDGEGAP_APPLICATION"]
	if !ok {
		return nil, runtime.NewError("EDGEGAP_APPLICATION not found in environment", 3)
	}

	// Check if dynamic versioning is enabled
	dynamicVersioning := false
	if dynamicVersioningStr, ok := env["EDGEGAP_DYNAMIC_VERSIONING"]; ok {
		dynamicVersioning, _ = strconv.ParseBool(dynamicVersioningStr)
	}

	var version string
	if dynamicVersioning {
		// Dynamic versioning enabled - version will be loaded from storage
		version = "DYNAMIC"
	} else {
		// Static versioning - require EDGEGAP_VERSION environment variable
		version, ok = env["EDGEGAP_VERSION"]
		if !ok {
			return nil, runtime.NewError("EDGEGAP_VERSION not found in environment (set EDGEGAP_DYNAMIC_VERSIONING=true to use dynamic versioning)", 3)
		}
	}

	portName, ok := env["EDGEGAP_PORT_NAME"]
	if !ok {
		return nil, runtime.NewError("EDGEGAP_PORT_NAME not found in environment", 3)
	}

	nakamaAccessUrl, ok := env["NAKAMA_ACCESS_URL"]
	if !ok {
		return nil, runtime.NewError("NAKAMA_ACCESS_URL not found in environment", 3)
	}

	pollingInterval, ok := env["EDGEGAP_POLLING_INTERVAL"]
	if !ok {
		pollingInterval = "15m"
	}

	cleanupInterval, ok := env["NAKAMA_CLEANUP_INTERVAL"]
	if !ok {
		cleanupInterval = "1m"
	}

	reservationMaxDuration, ok := env["NAKAMA_RESERVATION_MAX_DURATION"]
	if !ok {
		reservationMaxDuration = "30s"
	}

	mc := EdgegapManagerConfiguration{
		NakamaNode:             nakamaNode,
		ApiUrl:                 url,
		ApiToken:               token,
		Application:            app,
		Version:                version,
		DynamicVersioning:      dynamicVersioning,
		PortName:               portName,
		NakamaAccessUrl:        nakamaAccessUrl,
		PollingInterval:        pollingInterval,
		CleanupInterval:        cleanupInterval,
		ReservationMaxDuration: reservationMaxDuration,
	}

	err := mc.Validate()
	if err != nil {
		return nil, runtime.NewError(err.Error(), 3)
	}

	return &mc, nil
}

// Validate Will check if the configuration is valid
func (emc *EdgegapManagerConfiguration) Validate() error {
	errs := make([]error, 0)

	if emc.NakamaNode == "" {
		errs = append(errs, errors.New("nakama node must be set"))
	}

	if emc.ApiUrl == "" {
		errs = append(errs, errors.New("edgegap url must be set"))
	}

	if emc.ApiToken == "" {
		errs = append(errs, errors.New("edgegap token must be set"))
	}

	if emc.Application == "" {
		errs = append(errs, errors.New("edgegap application must be set"))
	}

	if emc.Version == "" {
		errs = append(errs, errors.New("edgegap application version must be set"))
	}

	if emc.PortName == "" {
		errs = append(errs, errors.New("edgegap application port name must be set"))
	}

	if emc.NakamaAccessUrl == "" {
		errs = append(errs, errors.New("nakama access url must be set"))
	}

	if _, err := time.ParseDuration(emc.PollingInterval); err != nil {
		errs = append(errs, errors.New("invalid polling interval: "+emc.PollingInterval))
	}

	if _, err := time.ParseDuration(emc.CleanupInterval); err != nil {
		errs = append(errs, errors.New("invalid cleanup interval: "+emc.CleanupInterval))
	}

	if _, err := time.ParseDuration(emc.ReservationMaxDuration); err != nil {
		errs = append(errs, errors.New("invalid reservation max duration: "+emc.ReservationMaxDuration))
	}

	// For static versioning, validate the version at startup
	// For dynamic versioning, validation happens when setting the version via update_edgegap_version RPC
	if !emc.DynamicVersioning {
		// Check with Edgegap if App Version Exists while testing the Token and the URL of the API
		apiHelper := helpers.NewAPIClient(emc.ApiUrl, emc.ApiToken)
		reply, err := apiHelper.Get(fmt.Sprintf("/v1/app/%s/version/%s", emc.Application, emc.Version))
		if err != nil {
			errs = append(errs, errors.New(fmt.Sprintf("Failed to get version from Edgegap API, this can happens if the URL is wrong: %s", err.Error())))
		}

		if reply != nil && reply.StatusCode != http.StatusOK {
			errs = append(errs, errors.New(fmt.Sprintf("Failed to validate version from Edgegap API, check token and if the Application/Version combo exists - Status Code=%s", reply.Status)))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
