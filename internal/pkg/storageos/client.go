package storageos

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/storageos/api-manager/internal/pkg/secret"
	"github.com/storageos/api-manager/internal/pkg/storageos/metrics"
	api "github.com/storageos/go-api/v2"
)

// Client provides access to the StorageOS API.
type Client struct {
	api       *api.APIClient
	transport http.RoundTripper
	ctx       context.Context
	traced    bool
}

const (
	// DefaultPort is the default api port.
	DefaultPort = 5705

	// DefaultScheme is used for api endpoint.
	DefaultScheme = "http"

	// TLSScheme scheme can be used if the api endpoint has TLS enabled.
	TLSScheme = "https"
)

var (
	// ErrNotInitialized is returned if the API client was accessed before it
	// was initialised.
	ErrNotInitialized = errors.New("api client not initialized")
	// ErrNoAuthToken is returned when the API client did not get an error
	// during authentication but no valid auth token was returned.
	ErrNoAuthToken = errors.New("no token found in auth response")

	// HTTPTimeout is the time limit for requests made by the API Client. The
	// timeout includes connection time, any redirects, and reading the response
	// body. The timer remains running after Get, Head, Post, or Do return and
	// will interrupt reading of the Response.Body.
	HTTPTimeout = 10 * time.Second

	// AuthenticationTimeout is the time limit for authentication requests to
	// complete.  It should be longer than the HTTPTimeout.
	AuthenticationTimeout = 20 * time.Second

	// DefaultRequestTimeout is the default time limit for api requests to
	// complete.  It should be longer than the HTTPTimeout.
	DefaultRequestTimeout = 20 * time.Second
)

// New returns a pre-authenticated client for the StorageOS API.  The
// authentication token must be refreshed periodically using
// AuthenticateRefresh().
func New(username, password, endpoint string) (*Client, error) {
	transport := http.DefaultTransport
	ctx, client, err := newAuthenticatedClient(username, password, endpoint, transport)
	if err != nil {
		return nil, err
	}
	return &Client{api: client, transport: transport, ctx: ctx}, nil
}

// NewTracedClient returns a pre-authenticated client for the StorageOS API that
// has tracing enabled.  The authentication token must be refreshed periodically
// using AuthenticateRefresh().
func NewTracedClient(username, password, endpoint string) (*Client, error) {
	metrics.RegisterMetrics()
	transport := metrics.InstrumentedTransport(http.DefaultTransport)
	ctx, client, err := newAuthenticatedClient(username, password, endpoint, transport)
	if err != nil {
		return nil, err
	}
	return &Client{api: client, transport: transport, ctx: ctx, traced: true}, nil
}

func newAuthenticatedClient(username, password, endpoint string, transport http.RoundTripper) (context.Context, *api.APIClient, error) {
	config := api.NewConfiguration()

	if !strings.Contains(endpoint, "://") {
		endpoint = fmt.Sprintf("%s://%s", DefaultScheme, endpoint)
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, nil, err
	}

	config.Scheme = u.Scheme
	config.Host = u.Host
	if !strings.Contains(u.Host, ":") {
		config.Host = fmt.Sprintf("%s:%d", u.Host, DefaultPort)
	}

	httpc := &http.Client{
		Timeout:   HTTPTimeout,
		Transport: transport,
	}
	config.HTTPClient = httpc

	// Get a wrappered API client.
	client := api.NewAPIClient(config)

	// Authenticate and return context with credentials and client.
	ctx, err := Authenticate(client, username, password)
	if err != nil {
		return nil, nil, err
	}

	return ctx, client, nil
}

// Authenticate against the API and set the authentication token in the client
// to be used for subsequent API requests.  The token must be refreshed
// periodically using AuthenticateRefresh().
func Authenticate(client *api.APIClient, username, password string) (context.Context, error) {
	// Create context just for the login.
	ctx, cancel := context.WithTimeout(context.Background(), AuthenticationTimeout)
	defer cancel()

	// Initial basic auth to retrieve the jwt token.
	_, resp, err := client.DefaultApi.AuthenticateUser(ctx, api.AuthUserData{
		Username: username,
		Password: password,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Set auth token in a new context for re-use.
	token := respAuthToken(resp)
	if token == "" {
		return nil, ErrNoAuthToken
	}
	return context.WithValue(context.Background(), api.ContextAccessToken, token), nil
}

// Refresh the api token on a given interval, or reset is received on the reset
// channel.  This function is blocking and is intended to be run in a goroutine.
// Errors are currently logged at info level since they will be retried and
// should be recoverable.
func (c *Client) Refresh(ctx context.Context, secretPath, endpoint string, reset <-chan struct{}, interval time.Duration, resultCounter metrics.ResultMetric, log logr.Logger) error {
	if c.api == nil || c.transport == nil {
		return ErrNotInitialized
	}
	for {
		select {
		case <-time.After(interval):
			// Refresh api token before it expires.  Default is 5 minute expiry.
			// Refresh will fail if the token has already expired.
			_, resp, err := c.api.DefaultApi.RefreshJwt(c.ctx)
			if err != nil {
				log.Info("failed to refresh storageos api credentials", "error", err)
				if c.traced {
					resultCounter.Increment("refresh_token", GetAPIErrorRootCause(err))
				}
				continue
			}
			defer resp.Body.Close()
			token := respAuthToken(resp)
			if token == "" {
				log.Info("failed to refresh storageos api credentials", "error", ErrNoAuthToken)
				if c.traced {
					resultCounter.Increment("refresh_token", ErrNoAuthToken)
				}
				continue
			}
			c.ctx = context.WithValue(c.ctx, api.ContextAccessToken, token)
			if c.traced {
				resultCounter.Increment("refresh_token", nil)
			}
		case <-reset:
			username, password, err := ReadCredsFromMountedSecret(secretPath)
			if err != nil {
				resultCounter.Increment("reset_api", err)
				continue
			}
			// Create new api client on any api errors.
			clientCtx, api, err := newAuthenticatedClient(username, password, endpoint, c.transport)
			if err != nil {
				log.Info("failed to recreate storageos api client", "error", err)
				if c.traced {
					resultCounter.Increment("reset_api", GetAPIErrorRootCause(err))
				}
				continue
			}
			c.api = api
			c.ctx = clientCtx
			if c.traced {
				resultCounter.Increment("reset_api", nil)
			}
		case <-ctx.Done():
			// Clean shutdown.
			return ctx.Err()
		}
	}
}

// respAuthToken is a helper to pull the auth token out of a HTTP Response.
func respAuthToken(resp *http.Response) string {
	if value := resp.Header.Get("Authorization"); value != "" {
		// "Bearer aaaabbbbcccdddeeeff"
		return strings.Split(value, " ")[1]
	}
	return ""
}

// ReadCredsFromMountedSecret reads the api username and password from a
// Kubernetes secret mounted at the given path.  If the username or password in
// the secret changes, the data in the mounted file will also change.
func ReadCredsFromMountedSecret(path string) (string, string, error) {
	username, err := secret.Read(filepath.Join(path, "username"))
	if err != nil {
		return "", "", err
	}
	password, err := secret.Read(filepath.Join(path, "password"))
	if err != nil {
		return "", "", err
	}
	return username, password, nil
}

// GetAPIErrorResponse returns the actual API response error incl. the response
// Body.
func GetAPIErrorResponse(oerr error) error {
	if n, ok := oerr.(api.GenericOpenAPIError); ok {
		return fmt.Errorf("%s: %s", strings.TrimSuffix(n.Error(), "\n"), n.Body())
	}
	return oerr
}

// GetAPIErrorRootCause attempts to unwrap the error to isolate the root cause,
// without decoration from the chain of calling functions.
//
// The list of error types evaluated is somewhat arbitrary: we want to capture
// things like:
//
// - `connect: connection refused`
// - `401 Unauthorized`
//
// But not:
//
//  - `Get http://storageos:5705/v2/namespaces: net/http: request canceled while
//     waiting for connection (Client.Timeout exceeded while awaiting headers)`
//
// Some errors could be unwrapped even further, but after a certain level the
// detail no longer makes sense.  This is purely subjective.
//
// Callers should not rely on specific errors being returned as they are subject
// to fine-tuning.
func GetAPIErrorRootCause(oerr error) error {
	if uerr, ok := oerr.(*url.Error); ok {
		uerrp := uerr.Unwrap()
		if uerrp == nil {
			return uerr
		}
		if nerr, ok := uerrp.(*net.OpError); ok {
			nerrp := nerr.Unwrap()
			if nerrp == nil {
				return nerr
			}
			return nerrp
		}
		return uerrp
	}
	return oerr
}
