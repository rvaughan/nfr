// Package client provides functions to handle AlphaSOC public API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/alphasoc/nfr/version"
	"golang.org/x/net/context/ctxhttp"
)

// Client interface for AlphaSOC API.
type Client interface {
	AccountRegister(*AccountRegisterRequest) error
	AccountStatus() (*AccountStatusResponse, error)
	Events(string) (*EventsResponse, error)
	Queries(*QueriesRequest) (*QueriesResponse, error)
	KeyRequest() (*KeyRequestResponse, error)
	KeyReset(*KeyResetRequest) error
}

// ErrorResponse represents AlphaSOC API error response.
type ErrorResponse struct {
	Message string `json:"message"`
}

// ErrNoAPIKey is returned when Client method is called without
// api key set if it's required.
var ErrNoAPIKey = errors.New("no api key")

// DefaultVersion for AlphaSOC API.
const DefaultVersion = "v1"

// default user agent for nfr.
const defaultUserAgent = "AlphaSOC NFR/" + version.Version

// AlphaSOCClient handles connection to AlphaSOC server.
type AlphaSOCClient struct {
	host    string
	client  *http.Client
	version string
	key     string
}

// New creates new AlphaSOC client with given host.
// It also sets timeout to 30 seconds.
func New(host, key string) *AlphaSOCClient {
	return &AlphaSOCClient{
		client:  &http.Client{},
		host:    strings.TrimSuffix(host, "/"),
		version: DefaultVersion,
		key:     key,
	}
}

// SetKey sets API key.
func (c *AlphaSOCClient) SetKey(key string) {
	c.key = key
}

// CheckKey check if client has valid AlphaSOC key.
func (c *AlphaSOCClient) CheckKey() error {
	_, err := c.AccountStatus()
	return err
}

// getAPIPath returns the versioned request path to call the api.
// It appends the query parameters to the path if they are not empty.
func (c *AlphaSOCClient) getAPIPath(path string, query url.Values) string {
	if query == nil {
		return fmt.Sprintf("%s/%s/%s", c.host, c.version, path)
	}
	return fmt.Sprintf("%s/%s/%s?%s", c.host, c.version, path, query.Encode())
}

func (c *AlphaSOCClient) get(ctx context.Context, path string, query url.Values) (*http.Response, error) {
	return c.do(ctx, http.MethodGet, path, query, nil, nil)
}

func (c *AlphaSOCClient) post(ctx context.Context, path string, query url.Values, obj interface{}) (*http.Response, error) {
	var buffer bytes.Buffer
	headers := http.Header{
		"Content-Type": []string{"application/json"},
	}
	if obj != nil {
		if err := json.NewEncoder(&buffer).Encode(obj); err != nil {
			return nil, err
		}
	}
	return c.do(ctx, http.MethodPost, path, query, &buffer, headers)
}

func (c *AlphaSOCClient) do(ctx context.Context, method, path string, query url.Values, body io.Reader, headers http.Header) (*http.Response, error) {
	fullPath := c.getAPIPath(path, query)
	req, err := http.NewRequest(method, fullPath, body)
	if err != nil {
		return nil, err
	}
	if c.key != "" {
		req.SetBasicAuth(c.key, "")
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	for key, value := range headers {
		req.Header[key] = value
	}

	resp, err := ctxhttp.Do(ctx, c.client, req)
	if err != nil {
		if err == context.DeadlineExceeded {
			return nil, fmt.Errorf("%s %s i/o timeout", method, fullPath)
		}
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		var errorResponse ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			return nil, err
		}
		return nil, errors.New(errorResponse.Message)
	}
	return resp, nil
}
