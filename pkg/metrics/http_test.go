package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestHTTPMetricsUseBoundedLabels(t *testing.T) {
	registry := prometheus.NewRegistry()
	httpMetrics, err := NewHTTP(registry)
	if err != nil {
		t.Fatalf("NewHTTP: %v", err)
	}

	httpMetrics.ObserveRequest("CUSTOM-UNBOUNDED-METHOD", "", http.StatusNotFound, time.Millisecond)
	recorder := httptest.NewRecorder()
	httpMetrics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()

	if !strings.Contains(body, `method="OTHER"`) {
		t.Fatalf("bounded method label missing: %s", body)
	}
	if !strings.Contains(body, `route="unmatched"`) {
		t.Fatalf("bounded route label missing: %s", body)
	}
	if !strings.Contains(body, `status_class="4xx"`) {
		t.Fatalf("status class label missing: %s", body)
	}
	if strings.Contains(body, "CUSTOM-UNBOUNDED-METHOD") {
		t.Fatalf("unbounded method leaked into metrics: %s", body)
	}
}

func TestProtectedHandlerRequiresOperatorToken(t *testing.T) {
	inner := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("metrics"))
	})

	tests := []struct {
		name       string
		configured string
		authority  string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "unset token disables endpoint",
			configured: "",
			authority:  "Bearer operator-secret",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing token",
			configured: "operator-secret",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "wrong token",
			configured: "operator-secret",
			authority:  "Bearer wrong-secret",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "correct token",
			configured: "operator-secret",
			authority:  "Bearer operator-secret",
			wantStatus: http.StatusOK,
			wantBody:   "metrics",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if test.authority != "" {
				request.Header.Set("Authorization", test.authority)
			}
			recorder := httptest.NewRecorder()

			ProtectedHandler(inner, test.configured).ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if body := recorder.Body.String(); test.wantBody != "" && body != test.wantBody {
				t.Fatalf("body = %q, want %q", body, test.wantBody)
			}
		})
	}
}
