// Package transport implements the typed HTTP client used to call the
// SoftSentry backend.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// Version is the agent's build version. It is a var (not a const) so a release
// build can stamp the real version into it via the linker, e.g.:
//
//	go build -ldflags "-X github.com/softsentry/agent/internal/transport.Version=1.2.0"
//
// This MUST match the version recorded for this binary in the backend's
// agent-binary manifest.json. If the manifest claims a higher version than the
// value baked in here, the backend offers an "update" the running binary can
// never satisfy — every restart re-downloads and re-applies the same binary,
// producing an endless restart loop. The build tooling (Makefile / build.ps1)
// stamps this value and writes the matching manifest from the same source.
var Version = "0.1.0"

// Client wraps net/http with timeouts and SoftSentry-specific helpers.
type Client struct {
	baseURL    string
	bearer     string
	httpClient *http.Client
}

// New returns a Client targeting the given server. token may be empty for
// pre-enroll calls.
func New(serverURL, bearerToken string) *Client {
	return &Client{
		baseURL: strings.TrimRight(serverURL, "/"),
		bearer:  bearerToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// APIError represents a non-2xx response.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("server returned %d: %s", e.Status, e.Body)
}

func (c *Client) do(
	ctx context.Context,
	method, path string,
	body any,
	out any,
) error {
	return c.doWithHeaders(ctx, method, path, body, out, nil)
}

func (c *Client) doWithHeaders(
	ctx context.Context,
	method, path string,
	body any,
	out any,
	headers map[string]string,
) error {
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(
		ctx, method, c.baseURL+path, bodyReader,
	)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set(
		"User-Agent",
		fmt.Sprintf("softsentry-agent/%s (%s/%s)", Version, runtime.GOOS, runtime.GOARCH),
	)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return &APIError{Status: resp.StatusCode, Body: string(data)}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// EnrollRequest is the agent → server enroll payload.
type EnrollRequest struct {
	EnrollmentToken string `json:"enrollment_token"`
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	OSVersion       string `json:"os_version"`
	Arch            string `json:"arch"`
	AgentVersion    string `json:"agent_version"`
}

// EnrollResponse is the server → agent enroll reply.
type EnrollResponse struct {
	MachineUUID       string `json:"machine_uuid"`
	AgentToken        string `json:"agent_token"`
	ScanIntervalHours int    `json:"scan_interval_hours"`
}

// Enroll exchanges a one-time enrollment token for a permanent agent token.
func (c *Client) Enroll(ctx context.Context, req EnrollRequest) (*EnrollResponse, error) {
	var out EnrollResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/agents/enroll", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AgentUpdate describes a newer agent binary the server is offering
// (heartbeat agent_update_available / GET binary/latest).
type AgentUpdate struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	SHA256      string `json:"sha256"`
}

// HeartbeatResponse carries the server's reply flags (protocol §2).
// AgentUpdateAvailable is nil when no update is offered.
type HeartbeatResponse struct {
	ConfigChanged        bool         `json:"config_changed"`
	ManualScanRequested  bool         `json:"manual_scan_requested"`
	AgentUpdateAvailable *AgentUpdate `json:"agent_update_available"`
}

// Heartbeat sends a liveness ping and returns the server's flags.
func (c *Client) Heartbeat(ctx context.Context, uptimeSeconds int64) (*HeartbeatResponse, error) {
	if c.bearer == "" {
		return nil, errors.New("heartbeat requires bearer token")
	}
	body := map[string]any{"agent_version": Version, "uptime_seconds": uptimeSeconds}
	var out HeartbeatResponse
	if err := c.do(ctx, http.MethodPost, "/api/v1/agents/heartbeat", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SignatureItem mirrors backend SignatureIn.
type SignatureItem struct {
	Status         string `json:"status"`
	Signer         string `json:"signer,omitempty"`
	Issuer         string `json:"issuer,omitempty"`
	CertThumbprint string `json:"cert_thumbprint,omitempty"`
}

// SoftwareItem mirrors backend SoftwareIn.
type SoftwareItem struct {
	Name          string         `json:"name"`
	Version       string         `json:"version"`
	Publisher     string         `json:"publisher,omitempty"`
	InstallDate   string         `json:"install_date,omitempty"`
	InstallPath   string         `json:"install_path,omitempty"`
	InstallSizeKB int64          `json:"install_size_kb,omitempty"`
	Arch          string         `json:"arch,omitempty"`
	Source        string         `json:"source"`
	Signature     *SignatureItem `json:"signature,omitempty"`
}

// ScanRequest mirrors backend ScanIn (POST /api/v1/agents/scans).
type ScanRequest struct {
	StartedAt   string         `json:"started_at"`
	CompletedAt string         `json:"completed_at"`
	ScanType    string         `json:"scan_type"`
	Trigger     string         `json:"trigger,omitempty"`
	Software    []SoftwareItem `json:"software"`
}

// ScanAccepted is the 202 reply (contains the canonical scan UUID).
type ScanAccepted struct {
	ScanUUID string `json:"scan_uuid"`
}

// PostScan uploads a scan result. idempotencyKey, if set, lets the server
// de-dup retries within 24h (protocol "Idempotency").
func (c *Client) PostScan(
	ctx context.Context, req ScanRequest, idempotencyKey string,
) (*ScanAccepted, error) {
	if c.bearer == "" {
		return nil, errors.New("scan upload requires bearer token")
	}
	var out ScanAccepted
	headers := map[string]string{}
	if idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}
	if err := c.doWithHeaders(
		ctx, http.MethodPost, "/api/v1/agents/scans", req, &out, headers,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBinary streams the agent binary at the given server path (e.g. the
// download_url from an AgentUpdate) into w. Used by the self-updater.
func (c *Client) GetBinary(ctx context.Context, path string, w io.Writer) error {
	if c.bearer == "" {
		return errors.New("binary download requires bearer token")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.bearer)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &APIError{Status: resp.StatusCode, Body: string(data)}
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("stream binary: %w", err)
	}
	return nil
}

// Health pings the unauthenticated /health endpoint.
func (c *Client) Health(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	if err := c.do(ctx, http.MethodGet, "/health", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
