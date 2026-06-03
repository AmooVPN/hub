package xui

import (
	"errors"
	"net"
	"net/http"
	"strings"
)

var (
	ErrXUIUnauthorized = errors.New("xui unauthorized")
	ErrXUIForbidden    = errors.New("xui forbidden")
	ErrXUINotFound     = errors.New("xui not found")
	ErrXUITimeout      = errors.New("xui timeout")
	ErrXUIUnavailable  = errors.New("xui unavailable")
	ErrXUIBadResponse  = errors.New("xui bad response")
)

type XUIError struct {
	PanelID    int64
	Operation  string
	StatusCode int
	Message    string
	Cause      error
}

func (e *XUIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return ErrXUIBadResponse.Error()
}

func (e *XUIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func normalizeStatusError(panelID int64, operation string, statusCode int) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return &XUIError{PanelID: panelID, Operation: operation, StatusCode: statusCode, Message: ErrXUIUnauthorized.Error(), Cause: ErrXUIUnauthorized}
	case http.StatusForbidden:
		return &XUIError{PanelID: panelID, Operation: operation, StatusCode: statusCode, Message: ErrXUIForbidden.Error(), Cause: ErrXUIForbidden}
	case http.StatusNotFound:
		return &XUIError{PanelID: panelID, Operation: operation, StatusCode: statusCode, Message: ErrXUINotFound.Error(), Cause: ErrXUINotFound}
	default:
		return &XUIError{PanelID: panelID, Operation: operation, StatusCode: statusCode, Message: ErrXUIBadResponse.Error(), Cause: ErrXUIBadResponse}
	}
}

func normalizeHTTPError(panelID int64, operation string, err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return &XUIError{PanelID: panelID, Operation: operation, Message: ErrXUITimeout.Error(), Cause: ErrXUITimeout}
	}
	if strings.Contains(msg, "timeout") {
		return &XUIError{PanelID: panelID, Operation: operation, Message: ErrXUITimeout.Error(), Cause: ErrXUITimeout}
	}
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host") {
		return &XUIError{PanelID: panelID, Operation: operation, Message: ErrXUIUnavailable.Error(), Cause: ErrXUIUnavailable}
	}
	return &XUIError{PanelID: panelID, Operation: operation, Message: ErrXUIUnavailable.Error(), Cause: err}
}
