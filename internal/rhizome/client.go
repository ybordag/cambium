// Package rhizome provides an HTTP client for Rhizome's internal API.
//
// Two surfaces:
//   - Agent: POST /internal/agent (non-streaming), POST /internal/agent/stream (SSE)
//   - Data:  GET/POST /internal/data/...
//
// Cambium injects user_id and provider_key into every request — Rhizome trusts
// these values because the internal interface is never exposed to the internet.
package rhizome

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Client calls Rhizome's internal FastAPI service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New returns a Client pointed at RHIZOME_INTERNAL_URL (default: http://localhost:8001).
func New() *Client {
	base := os.Getenv("RHIZOME_INTERNAL_URL")
	if base == "" {
		base = "http://localhost:8001"
	}
	return &Client{
		baseURL: base,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // agent turns can take a while
		},
	}
}

// AgentRequest is the body sent to /internal/agent and /internal/agent/stream.
type AgentRequest struct {
	UserID      string `json:"user_id"`
	ThreadID    string `json:"thread_id"`
	Message     string `json:"message"`
	Provider    string `json:"provider,omitempty"`
	ProviderKey string `json:"provider_key,omitempty"`
	Model       string `json:"model,omitempty"`
}

// AgentResponse is the body returned by /internal/agent.
type AgentResponse struct {
	ThreadID    string         `json:"thread_id"`
	Response    string         `json:"response"`
	Interaction map[string]any `json:"interaction,omitempty"`
}

// ResumeRequest is the body sent to /internal/agent/resume.
type ResumeRequest struct {
	UserID     string `json:"user_id"`
	ThreadID   string `json:"thread_id"`
	Resolution string `json:"resolution"`
}

// RunAgent calls POST /internal/agent and returns the complete response.
func (c *Client) RunAgent(req AgentRequest) (*AgentResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal agent request: %w", err)
	}
	resp, err := c.httpClient.Post(c.baseURL+"/internal/agent", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("call rhizome agent: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rhizome agent returned %d: %s", resp.StatusCode, b)
	}
	var out AgentResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode agent response: %w", err)
	}
	return &out, nil
}

// StreamAgent calls POST /internal/agent/stream and returns the raw SSE response
// body. The caller is responsible for closing it and forwarding to the client.
func (c *Client) StreamAgent(req AgentRequest) (io.ReadCloser, error) {
	return c.openStream("/internal/agent/stream", req)
}

// ResumeAgent calls POST /internal/agent/resume (non-streaming).
func (c *Client) ResumeAgent(req ResumeRequest) (*AgentResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal resume request: %w", err)
	}
	resp, err := c.httpClient.Post(c.baseURL+"/internal/agent/resume", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("call rhizome resume: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rhizome resume returned %d: %s", resp.StatusCode, b)
	}
	var out AgentResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode resume response: %w", err)
	}
	return &out, nil
}

// StreamResume calls POST /internal/agent/resume/stream and returns the SSE body.
func (c *Client) StreamResume(req ResumeRequest) (io.ReadCloser, error) {
	return c.openStream("/internal/agent/resume/stream", req)
}

// DataRequest proxies an arbitrary HTTP method to /internal/data/{path}.
// params (if any) are merged into the query string alongside user_id; payload
// (if non-nil) is JSON-encoded as the request body. Returns the raw response
// body — caller closes it.
func (c *Client) DataRequest(method, path, userID string, params url.Values, payload any) (io.ReadCloser, int, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("user_id", userID)
	u := fmt.Sprintf("%s/internal/data/%s?%s", c.baseURL, path, params.Encode())

	var bodyReader io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal data payload: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, u, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("build rhizome data request: %w", err)
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("rhizome data %s %s: %w", method, path, err)
	}
	return resp.Body, resp.StatusCode, nil
}

// DataGet proxies a GET to /internal/data/{path}?user_id=...&{params}.
func (c *Client) DataGet(path, userID string, params url.Values) (io.ReadCloser, int, error) {
	return c.DataRequest(http.MethodGet, path, userID, params, nil)
}

// DataPost proxies a POST to /internal/data/{path}?user_id=...
func (c *Client) DataPost(path, userID string, payload any) (io.ReadCloser, int, error) {
	return c.DataRequest(http.MethodPost, path, userID, nil, payload)
}

// DataDelete proxies a DELETE to /internal/data/{path}?user_id=...
func (c *Client) DataDelete(path, userID string) (io.ReadCloser, int, error) {
	return c.DataRequest(http.MethodDelete, path, userID, nil, nil)
}

// openStream posts JSON to a streaming endpoint and returns the response body
// without closing it — caller must close after proxying.
func (c *Client) openStream(endpoint string, payload any) (io.ReadCloser, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal stream request: %w", err)
	}
	// Streaming requests must not time out mid-stream — use a client without timeout.
	streamClient := &http.Client{}
	resp, err := streamClient.Post(c.baseURL+endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("open stream %s: %w", endpoint, err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("stream %s returned %d: %s", endpoint, resp.StatusCode, b)
	}
	return resp.Body, nil
}

// StreamData calls GET /internal/data/{path}?user_id=...&{params} and returns
// the raw streaming response body — caller must close it. Used for long-lived
// SSE GET endpoints on the data surface (e.g. the notification stream), as
// opposed to openStream which POSTs a JSON body to the agent surface.
func (c *Client) StreamData(path, userID string, params url.Values) (io.ReadCloser, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("user_id", userID)
	u := fmt.Sprintf("%s/internal/data/%s?%s", c.baseURL, path, params.Encode())

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build rhizome stream request: %w", err)
	}
	// Streaming requests must not time out mid-stream — use a client without timeout.
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open stream %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("stream %s returned %d: %s", path, resp.StatusCode, b)
	}
	return resp.Body, nil
}
