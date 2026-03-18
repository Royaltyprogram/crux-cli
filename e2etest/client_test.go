package e2etest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Royaltyprogram/aiops/dto/request"
	"github.com/Royaltyprogram/aiops/dto/response"
)

type Envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

type Client struct {
	BaseURL         string
	HTTP            *http.Client
	explicitBaseURL bool
	APIToken        string
	EnrollmentToken string
	AuthOrgID       string
	AuthUserID      string
}

func NewClientFromEnv() *Client {
	baseURL, ok := os.LookupEnv("E2E_BASE_URL")
	if !ok || baseURL == "" {
		baseURL = "http://127.0.0.1:8082"
	}
	apiToken := strings.TrimSpace(os.Getenv("E2E_API_TOKEN"))
	enrollmentToken := strings.TrimSpace(os.Getenv("E2E_CLI_TOKEN"))
	jar, _ := cookiejar.New(nil)
	return &Client{
		BaseURL: baseURL,
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
		},
		explicitBaseURL: ok && baseURL != "",
		APIToken:        apiToken,
		EnrollmentToken: enrollmentToken,
	}
}

func (c *Client) buildURL(path string, query url.Values) (string, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}
	ref, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parse path: %w", err)
	}
	out := u.ResolveReference(ref)
	if query != nil {
		out.RawQuery = query.Encode()
	}
	return out.String(), nil
}

func (c *Client) applyAuth(req *http.Request) {
	if req == nil {
		return
	}
	if strings.TrimSpace(req.Header.Get("X-AutoSkills-Token")) == "" && strings.TrimSpace(c.APIToken) != "" {
		req.Header.Set("X-AutoSkills-Token", c.APIToken)
	}
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	c.applyAuth(req)
	return c.HTTP.Do(req)
}

func (c *Client) Get(ctx context.Context, path string, query url.Values) (int, []byte, error) {
	fullURL, err := c.buildURL(path, query)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return 0, nil, err
	}
	rsp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer rsp.Body.Close()

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return rsp.StatusCode, nil, err
	}
	return rsp.StatusCode, body, nil
}

func (c *Client) TryAuthenticate(ctx context.Context) (bool, error) {
	if strings.TrimSpace(c.EnrollmentToken) != "" {
		fullURL, err := c.buildURL("/api/v1/auth/cli/login", nil)
		if err != nil {
			return false, err
		}

		body, err := json.Marshal(request.CLILoginReq{
			DeviceName: "e2e-device",
			Hostname:   "e2e.local",
			Platform:   "darwin/arm64",
			CLIVersion: "e2e",
			Tools:      []string{"codex"},
		})
		if err != nil {
			return false, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(body))
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-AutoSkills-Token", c.EnrollmentToken)

		rsp, err := c.HTTP.Do(req)
		if err != nil {
			return false, err
		}
		defer rsp.Body.Close()

		raw, err := io.ReadAll(rsp.Body)
		if err != nil {
			return false, err
		}
		env, data, err := decodeEnvelope[response.CLILoginResp](raw)
		if err != nil {
			return false, err
		}
		if rsp.StatusCode != http.StatusOK || env.Code != 0 || data == nil || strings.TrimSpace(data.AccessToken) == "" {
			return false, nil
		}

		c.APIToken = strings.TrimSpace(data.AccessToken)
		c.AuthOrgID = data.OrgID
		c.AuthUserID = data.UserID
		return true, nil
	}
	return strings.TrimSpace(c.APIToken) != "", nil
}

func postEnvelope[T any](ctx context.Context, c *Client, path string, payload any, withAuth bool) (*T, int, error) {
	fullURL, err := c.buildURL(path, nil)
	if err != nil {
		return nil, 0, err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if withAuth {
		c.applyAuth(req)
	}

	rsp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer rsp.Body.Close()

	raw, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, rsp.StatusCode, err
	}
	env, data, err := decodeEnvelope[T](raw)
	if err != nil {
		return nil, rsp.StatusCode, err
	}
	if rsp.StatusCode != http.StatusOK {
		return nil, rsp.StatusCode, fmt.Errorf("unexpected status %d: %s", rsp.StatusCode, string(raw))
	}
	if env.Code != 0 {
		return nil, rsp.StatusCode, fmt.Errorf("unexpected envelope code %d: %s", env.Code, string(raw))
	}
	if data == nil {
		return nil, rsp.StatusCode, fmt.Errorf("missing response data: %s", string(raw))
	}
	return data, rsp.StatusCode, nil
}

func decodeEnvelope[T any](body []byte) (Envelope, *T, error) {
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Envelope{}, nil, err
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return env, nil, nil
	}
	var data T
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return Envelope{}, nil, err
	}
	return env, &data, nil
}

type API struct {
	Health *HealthAPI
}

func NewAPI(c *Client) *API {
	return &API{
		Health: &HealthAPI{c: c},
	}
}

type HealthAPI struct {
	c *Client
}

func (h *HealthAPI) Get(ctx context.Context, message string) (int, Envelope, *response.HealthResp, error) {
	q := url.Values{}
	if message != "" {
		q.Set("message", message)
	}
	status, body, err := h.c.Get(ctx, "/health", q)
	if err != nil {
		return 0, Envelope{}, nil, err
	}
	env, data, err := decodeEnvelope[response.HealthResp](body)
	if err != nil {
		return status, Envelope{}, nil, err
	}
	return status, env, data, nil
}
