package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
