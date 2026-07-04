package tossinvest

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://openapi.tossinvest.com"
	DefaultTimeout = 15 * time.Second
)

type config struct {
	baseURL      string
	clientID     string
	clientSecret string
	accessToken  string
	accountSeq   string
	httpClient   *http.Client

	accountAPIsEnabled bool
	orderAPIsEnabled   bool
	liveTradingEnabled bool
}

func defaultConfig() config {
	return config{
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}
}

func (c config) validate() error {
	if strings.TrimSpace(c.baseURL) == "" {
		return &ConfigError{Field: "base_url", Message: "base URL is required"}
	}
	if _, err := url.ParseRequestURI(c.baseURL); err != nil {
		return &ConfigError{Field: "base_url", Message: err.Error()}
	}
	if c.httpClient == nil {
		return &ConfigError{Field: "http_client", Message: "HTTP client is required"}
	}
	if strings.TrimSpace(c.accessToken) == "" {
		if strings.TrimSpace(c.clientID) == "" {
			return &ConfigError{Field: "client_id", Message: "client ID is required when access token is not set"}
		}
		if strings.TrimSpace(c.clientSecret) == "" {
			return &ConfigError{Field: "client_secret", Message: "client secret is required when access token is not set"}
		}
	}
	return nil
}

type Option func(*config) error

func WithBaseURL(baseURL string) Option {
	return func(c *config) error {
		c.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		return nil
	}
}

func WithClientID(clientID string) Option {
	return func(c *config) error {
		c.clientID = strings.TrimSpace(clientID)
		return nil
	}
}

func WithClientSecret(clientSecret string) Option {
	return func(c *config) error {
		c.clientSecret = strings.TrimSpace(clientSecret)
		return nil
	}
}

func WithAccessToken(accessToken string) Option {
	return func(c *config) error {
		c.accessToken = strings.TrimSpace(accessToken)
		return nil
	}
}

func WithAccountSeq(accountSeq string) Option {
	return func(c *config) error {
		c.accountSeq = strings.TrimSpace(accountSeq)
		return nil
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *config) error {
		c.httpClient = httpClient
		return nil
	}
}

func WithAccountAPIsEnabled() Option {
	return func(c *config) error {
		c.accountAPIsEnabled = true
		return nil
	}
}

func WithOrderAPIsEnabled() Option {
	return func(c *config) error {
		c.orderAPIsEnabled = true
		return nil
	}
}

func WithLiveTradingEnabled() Option {
	return func(c *config) error {
		c.liveTradingEnabled = true
		return nil
	}
}
