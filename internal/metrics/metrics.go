package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var (
	// HTTPRequestsTotal counts HTTP requests by method, path pattern, and status code.
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration observes HTTP request duration in seconds by method and path.
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// ActiveSessions tracks the number of currently active agent sessions.
	ActiveSessions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "agentbox_active_sessions",
			Help: "Number of currently active agent sessions.",
		},
	)

	// RunsTotal counts agent runs by status (success, failure, timeout, etc.).
	RunsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentbox_runs_total",
			Help: "Total number of agent runs by status.",
		},
		[]string{"status"},
	)

	// WebSocketConnections tracks the number of active WebSocket connections.
	WebSocketConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "agentbox_websocket_connections",
			Help: "Number of active WebSocket connections.",
		},
	)

	// IMMessagesTotal counts incoming IM messages by platform (slack, discord, telegram, etc.).
	IMMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentbox_im_messages_total",
			Help: "Total number of IM messages received by platform.",
		},
		[]string{"platform"},
	)
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		ActiveSessions,
		RunsTotal,
		WebSocketConnections,
		IMMessagesTotal,
	)
}

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}
