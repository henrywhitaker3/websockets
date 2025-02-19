package websockets

import "github.com/prometheus/client_golang/prometheus"

type ClientMetrics struct {
	// The metric namespace
	Namespace string

	// The number of handlers registered
	HandlersRegistered prometheus.Counter

	// The total number of messages waiting for a response/ack
	Inflight prometheus.Gauge

	// Tracks time spent sending messages by topic
	MessagesSent *prometheus.HistogramVec

	// Tracks the time spent waiting for ack/reply
	ReplyTime *prometheus.HistogramVec

	// The total number of errors reading from the connection
	ReadErrors prometheus.Counter

	// The total number of errors writing to the connection
	WriteErrors prometheus.Counter

	// The total number of reconnections of the client
	Reconnections prometheus.Counter
}

func buildClientMetrics(m *ClientMetrics) {
	if m.HandlersRegistered == nil {
		m.HandlersRegistered = prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "websockets_handlers_registered",
			Help:      "The total number of handlers registered",
			Namespace: m.Namespace,
		})
	}
	if m.Inflight == nil {
		m.Inflight = prometheus.NewGauge(prometheus.GaugeOpts{
			Name:      "websockets_messages_inflight",
			Help:      "The total number of messages inflight waiting for a reply/ack",
			Namespace: m.Namespace,
		})
	}
	if m.MessagesSent == nil {
		m.MessagesSent = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:      "websockets_messages_sent_seconds",
				Help:      "The time in seconds taken to send a message",
				Namespace: m.Namespace,
			},
			[]string{"topic"},
		)
	}
	if m.ReplyTime == nil {
		m.ReplyTime = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:      "websockets_reply_time_seconds",
				Help:      "The total time spent waiting for a reply or an ack",
				Namespace: m.Namespace,
			},
			[]string{"topic"},
		)
	}
	if m.ReadErrors == nil {
		m.ReadErrors = prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "websockets_read_errors_total",
			Help:      "The total number of errors reading from the connection",
			Namespace: m.Namespace,
		})
	}
	if m.WriteErrors == nil {
		m.WriteErrors = prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "websockets_write_errors_total",
			Help:      "The total number of errors writing to the connection",
			Namespace: m.Namespace,
		})
	}
	if m.Reconnections == nil {
		m.Reconnections = prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "websockets_reconnections_total",
			Help:      "The number of times the client has reconnected to the server",
			Namespace: m.Namespace,
		})
	}
}
