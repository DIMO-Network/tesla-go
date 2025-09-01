package tesla

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// DefaultBaseURL is a base URL for the Tesla API that works in North
// America and Asia-Pacific.
//
// Most people should use their own deployment of Tesla's
// [vehicle-command].
//
// [vehicle-command]: https://github.com/teslamotors/vehicle-command
var DefaultBaseURL, _ = url.Parse("https://fleet-api.prd.na.vn.cloud.tesla.com")

type Client struct {
	hc      *http.Client
	baseURL *url.URL
}

type Option func(*Client)

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.hc = hc
	}
}

func WithBaseURL(u *url.URL) Option {
	return func(c *Client) {
		c.baseURL = u
	}
}

// New creates a new Tesla API client. Use options to supply different
// HTTP clients, base URLs, and so on.
func New(options ...Option) *Client {
	c := &Client{
		hc:      http.DefaultClient,
		baseURL: DefaultBaseURL,
	}

	for _, opt := range options {
		opt(c)
	}

	return c
}

type getFleetStatusRequest struct {
	VINs []string `json:"vins"`
}

type responseWrapper[A any] struct {
	Response A `json:"response"`
}

type vehicleInfo struct {
	VehicleCommandProtocolRequired     bool    `json:"vehicle_command_protocol_required"`
	SafetyScreenStreamingToggleEnabled *bool   `json:"safety_screen_streaming_toggle_enabled"`
	FirmwareVersion                    string  `json:"firmware_version"`
	FleetTelemetryVersion              *string `json:"fleet_telemetry_version"`
	TotalNumberOfKeys                  *int    `json:"total_number_of_keys"`
	DiscountedDeviceData               bool    `json:"discounted_device_data"`
}

type fleetStatus struct {
	KeyPairedVINs []string               `json:"key_paired_vins"`
	UnpairedVINs  []string               `json:"unpaired_vins"`
	VehicleInfo   map[string]vehicleInfo `json:"vehicle_info"`
}

// GetFleetStatus retrieves fleet status information for the car with
// the given VIN, using the [fleet_status] endpoint.
//
// [fleet_status](https://developer.tesla.com/docs/fleet-api/endpoints/vehicle-endpoints#fleet-status)
func (c *Client) GetFleetStatus(ctx context.Context, token, vin string) (*FleetStatus, error) {
	reqBody := getFleetStatusRequest{
		VINs: []string{vin}, // Recommended to only ever do one.
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request body: %w", err)
	}

	u := c.baseURL.JoinPath("api/1/vehicles/fleet_status")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	respBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close() //nolint:errcheck
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	var respBody responseWrapper[fleetStatus]
	err = json.Unmarshal(respBytes, &respBody)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}

	fs := respBody.Response.VehicleInfo[vin]

	return &FleetStatus{
		KeyPaired:                          len(respBody.Response.KeyPairedVINs) == 1, // Maybe validate more here.
		VehicleCommandProtocolRequired:     fs.VehicleCommandProtocolRequired,
		SafetyScreenStreamingToggleEnabled: fs.SafetyScreenStreamingToggleEnabled,
		FirmwareVersion:                    fs.FirmwareVersion,
		FleetTelemetryVersion:              fs.FleetTelemetryVersion,
		TotalNumberOfKeys:                  fs.TotalNumberOfKeys,
		DiscountedDeviceData:               fs.DiscountedDeviceData,
	}, nil
}

type FleetStatus struct {
	KeyPaired                          bool
	VehicleCommandProtocolRequired     bool
	SafetyScreenStreamingToggleEnabled *bool
	FirmwareVersion                    string
	FleetTelemetryVersion              *string
	TotalNumberOfKeys                  *int
	DiscountedDeviceData               bool
}
