package tossinvest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	tossapi "github.com/awuzag/tossinvest-go/internal/generated/tossapi"
)

func (c *Client) ExecuteTossAPI(ctx context.Context, request tossapi.Request, out any) error {
	if err := c.ensureOperationEnabled(request.OperationID); err != nil {
		return err
	}
	if request.Form != nil {
		return c.doForm(ctx, request, out)
	}
	if !request.ExpectEnvelope {
		return c.do(ctx, request.Method, request.Path, request.Query, request.AccountSeq, request.Body, out)
	}

	var envelope RawEnvelope
	if err := c.do(ctx, request.Method, request.Path, request.Query, request.AccountSeq, request.Body, &envelope); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	payload := envelope.Result
	if len(payload) == 0 {
		payload = []byte("null")
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return &DecodeError{Op: request.OperationID, Err: err}
	}
	return nil
}

func (c *Client) ensureOperationEnabled(operationID string) error {
	if isAccountOperation(operationID) && !c.accountAPIsEnabled {
		return &FeatureDisabledError{Feature: "account APIs", EnableWith: "WithAccountAPIsEnabled", Operation: operationID}
	}
	if isOrderOperation(operationID) {
		if !c.accountAPIsEnabled {
			return &FeatureDisabledError{Feature: "account APIs", EnableWith: "WithAccountAPIsEnabled", Operation: operationID}
		}
		if !c.orderAPIsEnabled {
			return &FeatureDisabledError{Feature: "order APIs", EnableWith: "WithOrderAPIsEnabled", Operation: operationID}
		}
	}
	if isLiveTradingOperation(operationID) && !c.liveTradingEnabled {
		return &FeatureDisabledError{Feature: "live trading", EnableWith: "WithLiveTradingEnabled", Operation: operationID}
	}
	return nil
}

func isAccountOperation(operationID string) bool {
	switch operationID {
	case tossapi.OperationGetAccounts,
		tossapi.OperationGetHoldings,
		tossapi.OperationGetBuyingPower,
		tossapi.OperationGetSellableQuantity,
		tossapi.OperationGetCommissions:
		return true
	default:
		return false
	}
}

func isOrderOperation(operationID string) bool {
	switch operationID {
	case tossapi.OperationGetOrders,
		tossapi.OperationGetOrder,
		tossapi.OperationCreateOrder,
		tossapi.OperationModifyOrder,
		tossapi.OperationCancelOrder:
		return true
	default:
		return false
	}
}

func isLiveTradingOperation(operationID string) bool {
	switch operationID {
	case tossapi.OperationCreateOrder,
		tossapi.OperationModifyOrder,
		tossapi.OperationCancelOrder:
		return true
	default:
		return false
	}
}

func (c *Client) doForm(ctx context.Context, request tossapi.Request, out any) error {
	endpoint := c.baseURL + request.Path
	req, err := http.NewRequestWithContext(ctx, request.Method, endpoint, strings.NewReader(request.Form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	req.Header.Set("accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var oauthErr OAuthError
		if json.Unmarshal(body, &oauthErr) == nil && oauthErr.ErrorCode != "" {
			oauthErr.StatusCode = resp.StatusCode
			return &oauthErr
		}
		return &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status, Body: string(body), RequestID: resp.Header.Get("X-Request-Id")}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return &DecodeError{Op: request.OperationID, Err: err}
	}
	return nil
}
