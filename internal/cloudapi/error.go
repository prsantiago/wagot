package cloudapi

import (
	"encoding/json"
	"fmt"
)

// APIError is an error returned by the Graph API. Inspect it with errors.As.
//
// See https://developers.facebook.com/docs/whatsapp/cloud-api/support/error-codes
type APIError struct {
	HTTPStatus   int    `json:"-"`
	Message      string `json:"message"`
	Type         string `json:"type"`
	Code         int    `json:"code"`
	ErrorSubcode int    `json:"error_subcode,omitempty"`
	FBTraceID    string `json:"fbtrace_id,omitempty"`
	ErrorData    struct {
		MessagingProduct string `json:"messaging_product,omitempty"`
		Details          string `json:"details,omitempty"`
	} `json:"error_data"`
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("cloudapi: %s (code %d, http %d)", e.Message, e.Code, e.HTTPStatus)
	if e.ErrorData.Details != "" {
		msg += ": " + e.ErrorData.Details
	}
	return msg
}

func parseError(status int, body []byte) error {
	var envelope struct {
		Error *APIError `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Error == nil {
		return fmt.Errorf("cloudapi: unexpected http %d: %.200s", status, body)
	}
	envelope.Error.HTTPStatus = status
	return envelope.Error
}
