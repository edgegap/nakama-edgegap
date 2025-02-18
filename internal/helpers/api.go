package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIClient struct
type APIClient struct {
	BaseURL    string
	AuthToken  string
	HTTPClient *http.Client
}

// NewAPIClient creates a new APIClient instance
func NewAPIClient(baseURL, token string) *APIClient {
	return &APIClient{
		BaseURL:   baseURL,
		AuthToken: token,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second, // Set timeout for requests
		},
	}
}

// request is a helper function to make HTTP requests
func (c *APIClient) request(method, endpoint string, payload interface{}) (*http.Response, error) {
	url := c.BaseURL + endpoint

	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("error marshalling payload: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers conditionally
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.AuthToken != "" {
		req.Header.Set("Authorization", c.AuthToken)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}

	return resp, nil
}

// Get makes a GET request
func (c *APIClient) Get(endpoint string) (*http.Response, error) {
	return c.request(http.MethodGet, endpoint, nil)
}

// Post makes a POST request with a JSON payload
func (c *APIClient) Post(endpoint string, payload interface{}) (*http.Response, error) {
	return c.request(http.MethodPost, endpoint, payload)
}

// Put makes a PUT request with a JSON payload
func (c *APIClient) Put(endpoint string, payload interface{}) (*http.Response, error) {
	return c.request(http.MethodPut, endpoint, payload)
}

// Patch makes a PATCH request with a JSON payload
func (c *APIClient) Patch(endpoint string, payload interface{}) (*http.Response, error) {
	return c.request(http.MethodPatch, endpoint, payload)
}

// Delete makes a DELETE request
func (c *APIClient) Delete(endpoint string) (*http.Response, error) {
	return c.request(http.MethodDelete, endpoint, nil)
}
