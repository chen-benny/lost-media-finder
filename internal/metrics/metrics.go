package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	PagesProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pages_processed",
		Help: "The total number of pages processed",
	})
	VideoFound = promauto.NewCounter(prometheus.CounterOpts{
		Name: "video_found",
		Help: "The total number of videos found",
	})
	TargetsFound = promauto.NewCounter(prometheus.CounterOpts{
		Name: "targets_found",
		Help: "The total number of targets found",
	})
	Errors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "errors",
		Help: "The total number of errors",
	})
	QueueSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "queue_size",
		Help: "Current queue size",
	})
	FetchDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "fetch_duration_seconds",
		Help:    "HTTP fetch duration",
		Buckets: []float64{0.1, 0.5, 1, 2, 5},
	})
)

func Serve(port string) {
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
