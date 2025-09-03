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
	InitialVersion         string `json:"initial_version"`
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

	// Get initial version (optional, used when no version exists in storage)
	initialVersion := env["INITIAL_EDGEGAP_VERSION"]
	
	// For backward compatibility, check EDGEGAP_VERSION if INITIAL_EDGEGAP_VERSION is not set
	if initialVersion == "" {
		initialVersion = env["EDGEGAP_VERSION"]
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
		InitialVersion:         initialVersion,
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

	// Initial version is optional - only used when no version exists in storage

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

	// Validate Edgegap API connection
	apiHelper := helpers.NewAPIClient(emc.ApiUrl, emc.ApiToken)
	// Test API connection by checking the application exists
	reply, err := apiHelper.Get(fmt.Sprintf("/v1/app/%s", emc.Application))
	if err != nil {
		errs = append(errs, errors.New(fmt.Sprintf("Failed to connect to Edgegap API, check URL: %s", err.Error())))
	} else if reply != nil && reply.StatusCode != http.StatusOK {
		errs = append(errs, errors.New(fmt.Sprintf("Failed to validate application with Edgegap API, check token and application name - Status Code=%s", reply.Status)))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
