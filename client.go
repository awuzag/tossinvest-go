package tossinvest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	tossapi "github.com/awuzag/tossinvest-go/internal/generated/tossapi"
)

type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	accountSeq   string
	httpClient   *http.Client

	accountAPIsEnabled bool
	orderAPIsEnabled   bool
	liveTradingEnabled bool

	mu    sync.RWMutex
	token Token
}

func New(options ...Option) (*Client, error) {
	cfg := defaultConfig()
	for _, option := range options {
		if option == nil {
			return nil, &ConfigError{Field: "option", Message: "option is required"}
		}
		if err := option(&cfg); err != nil {
			return nil, err
		}
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &Client{
		baseURL:            cfg.baseURL,
		clientID:           cfg.clientID,
		clientSecret:       cfg.clientSecret,
		accountSeq:         cfg.accountSeq,
		httpClient:         cfg.httpClient,
		accountAPIsEnabled: cfg.accountAPIsEnabled,
		orderAPIsEnabled:   cfg.orderAPIsEnabled,
		liveTradingEnabled: cfg.liveTradingEnabled,
		token:              Token{AccessToken: cfg.accessToken, TokenType: "Bearer"},
	}, nil
}

func (c *Client) UseToken(token Token) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

func (c *Client) currentAccessToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return strings.TrimSpace(c.token.AccessToken)
}

func (c *Client) Token(ctx context.Context) (Token, error) {
	token, err := tossapi.IssueOAuth2Token(ctx, c, tossapi.IssueOAuth2TokenRequest{
		Body: tossapi.OAuth2TokenRequest{
			GrantType:    "client_credentials",
			ClientID:     c.clientID,
			ClientSecret: c.clientSecret,
		},
	})
	if err != nil {
		return Token{}, err
	}
	c.UseToken(token)
	return token, nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, accountSeq string, out any) error {
	return c.do(ctx, http.MethodGet, path, query, accountSeq, nil, out)
}

func (c *Client) post(ctx context.Context, path string, accountSeq string, body any, out any) error {
	return c.do(ctx, http.MethodPost, path, nil, accountSeq, body, out)
}

func (c *Client) do(ctx context.Context, method string, path string, query url.Values, accountSeq string, body any, out any) error {
	if c.currentAccessToken() == "" {
		if _, err := c.Token(ctx); err != nil {
			return err
		}
	}

	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("accept", "application/json")
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	req.Header.Set("authorization", bearer(c.currentAccessToken()))
	if accountSeq = firstNonEmpty(accountSeq, c.accountSeq); accountSeq != "" {
		req.Header.Set("X-Tossinvest-Account", accountSeq)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if apiErr := decodeAPIError(resp.StatusCode, payload); apiErr != nil {
			return apiErr
		}
		return &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status, Body: string(payload), RequestID: resp.Header.Get("X-Request-Id")}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return &DecodeError{Op: method + " " + path, Err: err}
	}
	return nil
}

func bearer(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}
	return "Bearer " + token
}

func q(values ...string) url.Values {
	query := url.Values{}
	for i := 0; i+1 < len(values); i += 2 {
		if strings.TrimSpace(values[i+1]) != "" {
			query.Set(values[i], values[i+1])
		}
	}
	return query
}

func addInt(query url.Values, key string, value int) {
	if value > 0 {
		query.Set(key, strconv.Itoa(value))
	}
}

func addBool(query url.Values, key string, value *bool) {
	if value != nil {
		query.Set(key, strconv.FormatBool(*value))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func comma(values []string) string {
	return strings.Join(values, ",")
}

func nowTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), DefaultTimeout)
}

func _keepTimeImport(_ time.Time) {}
