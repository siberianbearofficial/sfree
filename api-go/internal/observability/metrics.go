package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	HTTPActiveRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_active_requests",
		Help: "Number of in-flight HTTP requests.",
	})

	ChunkUploadsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chunk_uploads_total",
		Help: "Total number of chunk upload operations.",
	}, []string{"status"})

	ChunkDownloadsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chunk_downloads_total",
		Help: "Total number of chunk download operations.",
	}, []string{"status"})

	ChunkDeletesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chunk_deletes_total",
		Help: "Total number of chunk delete operations.",
	}, []string{"status"})

	ChunkBytesUploaded = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "chunk_bytes_uploaded_total",
		Help: "Total bytes uploaded across all chunk operations.",
	})

	ChunkBytesDownloaded = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "chunk_bytes_downloaded_total",
		Help: "Total bytes downloaded across all chunk operations.",
	})
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		HTTPActiveRequests,
		ChunkUploadsTotal,
		ChunkDownloadsTotal,
		ChunkDeletesTotal,
		ChunkBytesUploaded,
		ChunkBytesDownloaded,
	)
}
