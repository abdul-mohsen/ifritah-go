package metrics

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HTTP struct {
	registry prometheus.Registerer
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewHTTP(registry prometheus.Registerer) (*HTTP, error) {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}

	requests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ifritah",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests completed by the API.",
		},
		[]string{"method", "route", "status_class"},
	)
	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "ifritah",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "route", "status_class"},
	)
	if err := registry.Register(requests); err != nil {
		return nil, err
	}
	if err := registry.Register(duration); err != nil {
		registry.Unregister(requests)
		return nil, err
	}

	return &HTTP{
		registry: registry,
		requests: requests,
		duration: duration,
	}, nil
}

func (h *HTTP) ObserveRequest(method, route string, status int, duration time.Duration) {
	if h == nil {
		return
	}
	method = boundedMethod(method)
	route = boundedRoute(route)
	statusClass := statusClass(status)
	h.requests.WithLabelValues(method, route, statusClass).Inc()
	h.duration.WithLabelValues(method, route, statusClass).Observe(duration.Seconds())
}

func (h *HTTP) Handler() http.Handler {
	if h == nil || h.registry == nil {
		return promhttp.Handler()
	}
	if gatherer, ok := h.registry.(prometheus.Gatherer); ok {
		return promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{})
	}
	return promhttp.Handler()
}

func ProtectedHandler(handler http.Handler, configuredToken string) http.Handler {
	configuredToken = strings.TrimSpace(configuredToken)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if configuredToken == "" {
			http.NotFound(writer, request)
			return
		}
		if !validBearerToken(request.Header.Get("Authorization"), configuredToken) {
			http.Error(writer, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		handler.ServeHTTP(writer, request)
	})
}

func validBearerToken(header, configuredToken string) bool {
	scheme, providedToken, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(strings.TrimSpace(scheme), "Bearer") {
		return false
	}
	providedToken = strings.TrimSpace(providedToken)
	if providedToken == "" {
		return false
	}

	expectedDigest := sha256.Sum256([]byte(configuredToken))
	providedDigest := sha256.Sum256([]byte(providedToken))
	return subtle.ConstantTimeCompare(expectedDigest[:], providedDigest[:]) == 1
}

func boundedMethod(method string) string {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return strings.ToUpper(method)
	default:
		return "OTHER"
	}
}

func boundedRoute(route string) string {
	if route == "" {
		return "unmatched"
	}
	if len(route) > 128 || !strings.HasPrefix(route, "/") {
		return "other"
	}
	return route
}

func statusClass(status int) string {
	switch {
	case status >= 100 && status < 200:
		return "1xx"
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "other"
	}
}
