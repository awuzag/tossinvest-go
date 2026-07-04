package tossinvest

import (
	"encoding/json"
	"fmt"
)

type ConfigError struct {
	Field   string
	Message string
}

func (err *ConfigError) Error() string {
	return fmt.Sprintf("tossinvest: config %s: %s", err.Field, err.Message)
}

type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
	RequestID  string
}

func (err *HTTPError) Error() string {
	if err.RequestID != "" {
		return fmt.Sprintf("tossinvest: http error: status=%d request_id=%s", err.StatusCode, err.RequestID)
	}
	return fmt.Sprintf("tossinvest: http error: status=%d", err.StatusCode)
}

type APIError struct {
	StatusCode int
	RequestID  string         `json:"requestId"`
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Data       map[string]any `json:"data,omitempty"`
}

func (err *APIError) Error() string {
	return fmt.Sprintf("tossinvest: api error: status=%d code=%s message=%s request_id=%s", err.StatusCode, err.Code, err.Message, err.RequestID)
}

type OAuthError struct {
	StatusCode       int
	ErrorCode        string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
}

func (err *OAuthError) Error() string {
	if err.ErrorDescription != "" {
		return fmt.Sprintf("tossinvest: oauth error: status=%d error=%s description=%s", err.StatusCode, err.ErrorCode, err.ErrorDescription)
	}
	return fmt.Sprintf("tossinvest: oauth error: status=%d error=%s", err.StatusCode, err.ErrorCode)
}

type DecodeError struct {
	Op  string
	Err error
}

func (err *DecodeError) Error() string {
	return fmt.Sprintf("tossinvest: decode %s: %v", err.Op, err.Err)
}

func (err *DecodeError) Unwrap() error {
	return err.Err
}

type FeatureDisabledError struct {
	Feature    string
	EnableWith string
	Operation  string
}

func (err *FeatureDisabledError) Error() string {
	if err.Operation != "" {
		return fmt.Sprintf("tossinvest: %s is disabled for %s; enable with %s", err.Feature, err.Operation, err.EnableWith)
	}
	return fmt.Sprintf("tossinvest: %s is disabled; enable with %s", err.Feature, err.EnableWith)
}

func decodeAPIError(status int, body []byte) error {
	var envelope struct {
		Error APIError `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.Code != "" {
		envelope.Error.StatusCode = status
		return &envelope.Error
	}
	return nil
}
