package handlers

import (
	"strings"
)

const (
	ZatcaOnboardStateNotStarted = "not_started"
	ZatcaOnboardStateQueued     = "queued"
	ZatcaOnboardStateCompliance = "compliance"
	ZatcaOnboardStateProduction = "production"
	ZatcaOnboardStateDone       = "done"
	ZatcaOnboardStateFailed     = "failed"
)

func zatcaOnboardingPublisherError(err error) (*string, *string) {
	code := "publisher_error"
	if err == nil {
		return &code, nil
	}

	message := strings.TrimSpace(err.Error())
	if len(message) > 1024 {
		message = message[:1024]
	}
	return &code, &message
}
