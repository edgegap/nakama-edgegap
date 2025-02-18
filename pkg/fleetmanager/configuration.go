package fleetmanager

import (
	"context"
	"errors"
	"fmt"
	"github.com/edgegap/nakama-edgegap/internal/helpers"
	"github.com/heroiclabs/nakama-common/runtime"
	"net/http"
	"strings"
)

type EdgegapManagerConfiguration struct {
	NakamaNode      string `json:"nakama_node"`
	ApiUrl          string `json:"base_url"`
	ApiToken        string `json:"api_token"`
	Application     string `json:"application"`
	Version         string `json:"version"`
	PortName        string `json:"port_name"`
	NakamaAccessUrl string `json:"nakama_access_url"`
	NakamaHttpKey   string `json:"nakama_http_key"`
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

	version, ok := env["EDGEGAP_VERSION"]
	if !ok {
		return nil, runtime.NewError("EDGEGAP_VERSION not found in environment", 3)
	}

	portName, ok := env["EDGEGAP_PORT_NAME"]
	if !ok {
		return nil, runtime.NewError("EDGEGAP_PORT_NAME not found in environment", 3)
	}

	nakamaAccessUrl, ok := env["NAKAMA_ACCESS_URL"]
	if !ok {
		return nil, runtime.NewError("NAKAMA_ACCESS_URL not found in environment", 3)
	}

	mc := EdgegapManagerConfiguration{
		NakamaNode:      nakamaNode,
		ApiUrl:          url,
		ApiToken:        token,
		Application:     app,
		Version:         version,
		PortName:        portName,
		NakamaAccessUrl: nakamaAccessUrl,
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

	// Check with Edgegap if App Version Exists while testing the Token and the URL of the API
	apiHelper := helpers.NewAPIClient(emc.ApiUrl, emc.ApiToken)
	reply, err := apiHelper.Get(fmt.Sprintf("/v1/app/%s/version/%s", emc.Application, emc.Version))
	if err != nil {
		errs = append(errs, errors.New(fmt.Sprintf("Failed to get version from Edgegap API, this can happens if the URL is wrong: %s", err.Error())))
	}

	if reply != nil && reply.StatusCode != http.StatusOK {
		errs = append(errs, errors.New(fmt.Sprintf("Failed to validate version from Edgegap API, check token and if the Application/Version combo exists - Status Code=%s", reply.Status)))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
