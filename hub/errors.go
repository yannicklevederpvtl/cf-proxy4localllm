package main

import (
	"context"
	"errors"
	"net/http"
)

func chatErrorHTTPStatus(err error) int {
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}
