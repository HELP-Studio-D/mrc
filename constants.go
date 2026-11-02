package mrc

import (
	cabi "github.com/Edestuss0/MMA-Network-protocol/cabi"
)

const (
	contextTokenKey = "authToken"
	contextClaimsKey = "authClaims"

	// All status codes for mrc/protocol answer
	StatusOK               = cabi.StatusOK
	StatusBadRequest       = cabi.StatusBadRequest
	StatusUnauthorized     = cabi.StatusUnauthorized
	StatusForbidden        = cabi.StatusForbidden
	StatusNotFound         = cabi.StatusNotFound
	StatusConflict         = cabi.StatusConflict
	StatusInternalError    = cabi.StatusInternalError
	StatusMessage          = cabi.StatusMessage
	StatusShutdown         = cabi.StatusShutdown
	StatusChannelConnected = cabi.StatusChannelConnected
	StatusTimeout          = cabi.StatusTimeout
)