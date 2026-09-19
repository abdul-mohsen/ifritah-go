package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ifritah/web-service-gin/pkg/logging"
)

func TestVINUpstreamEventUsesRequestContextAndOmitsResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"vin":"ABC123","token":"response-secret"}`))
	}))
	defer server.Close()

	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(logging.New(logging.Config{Level: slog.LevelInfo, Format: "json"}, &output))
	t.Cleanup(func() { slog.SetDefault(previous) })

	ctx := logging.WithRequestID(context.Background(), "req-vin")
	body, err := getBody(ctx, server.URL)
	if err != nil {
		t.Fatalf("getBody: %v", err)
	}
	if !bytes.Contains(body, []byte("response-secret")) {
		t.Fatalf("test upstream body was not returned")
	}

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode VIN event: %v", err)
	}
	if record["msg"] != "vin.upstream_response_received" ||
		record["request_id"] != "req-vin" ||
		record["status"] != float64(http.StatusOK) {
		t.Fatalf("VIN event = %#v", record)
	}
	if strings.Contains(output.String(), "response-secret") ||
		strings.Contains(output.String(), `"body"`) {
		t.Fatalf("VIN response body leaked into event: %s", output.String())
	}
}

func TestVINUpstreamPropagatesValidatedRequestIDAndReturnsTypedStatusError(t *testing.T) {
	var requestID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestID = request.Header.Get(logging.RequestIDHeader)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	ctx := logging.WithRequestID(context.Background(), "req-vin-status")
	_, err := getBody(ctx, server.URL)
	var statusErr *VINUpstreamStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("getBody error = %v, want typed 502 error", err)
	}
	if requestID != "req-vin-status" {
		t.Fatalf("upstream request ID = %q, want req-vin-status", requestID)
	}
}

func TestVINUpstreamResponseIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), maxVINResponseBytes+1))
	}))
	defer server.Close()

	_, err := getBody(context.Background(), server.URL)
	var sizeErr *VINResponseTooLargeError
	if !errors.As(err, &sizeErr) || sizeErr.Limit != maxVINResponseBytes {
		t.Fatalf("getBody error = %v, want bounded response error", err)
	}
}

func TestVINUpstreamUsesExplicitTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	startedAt := time.Now()
	_, err := getBodyWithTimeout(context.Background(), server.URL, 20*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("getBody error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("bounded request took %s", elapsed)
	}
}

func TestIsValidVIN(t *testing.T) {
	for _, value := range []string{
		"1M8GDM9AXKP042788",
		"jhmcm56557c404453",
	} {
		if !isValidVIN(strings.ToUpper(value)) {
			t.Fatalf("isValidVIN(%q) = false", value)
		}
	}
	for _, value := range []string{
		"",
		"1M8GDM9AXKP04278",
		"1M8GDM9AXKP0427880",
		"1M8GDM9AOKP042788",
		"1M8GDM9A/KP042788",
	} {
		if isValidVIN(value) {
			t.Fatalf("isValidVIN(%q) = true", value)
		}
	}
}
