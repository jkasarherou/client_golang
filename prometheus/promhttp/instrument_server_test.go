// Copyright 2016 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promhttp

import (
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMiddlewareAPI(t *testing.T) {
	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "in_flight_requests",
		Help: "A gauge of requests currently being served by the wrapped handler.",
	})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"code", "method"},
	)

	histVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "response_duration_seconds",
			Help:    "A histogram of request latencies.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	responseSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "push_request_size_bytes",
			Help:    "A histogram of request sizes for requests.",
			Buckets: []float64{200, 500, 900, 1500},
		},
		[]string{},
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	prometheus.MustRegister(inFlightGauge, counter, histVec, responseSize)

	chain := InstrumentHandlerInFlight(inFlightGauge,
		InstrumentHandlerCounter(counter,
			InstrumentHandlerDuration(histVec,
				InstrumentHandlerResponseSize(responseSize, handler),
			),
		),
	)

	r, _ := http.NewRequest("GET", "www.example.com", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, r)
}

func ExampleInstrumentHandlerDuration() {
	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "in_flight_requests",
		Help: "A gauge of requests currently being served by the wrapped handler.",
	})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"code", "method"},
	)

	// pushVec is partitioned with custom buckets based on expected request
	// duration.
	pushVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "push_duration_seconds",
			Help:        "A histogram of latencies for requests to the push handler.",
			Buckets:     []float64{.25, .5, 1, 2.5, 5, 10},
			ConstLabels: prometheus.Labels{"handler": "push"},
		},
		[]string{"method"},
	)

	// pullVec is partitioned with custom buckets based on expected request
	// duration, which differ from those defined in pushVec.
	pullVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "pull_duration_seconds",
			Help:        "A histogram of latencies for requests to the pull handler.",
			Buckets:     []float64{.005, .01, .025, .05},
			ConstLabels: prometheus.Labels{"handler": "pull"},
		},
		[]string{"method"},
	)

	// responseSize is an ObserverVec partitioned with no instance labels.
	responseSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "push_request_size_bytes",
			Help:    "A histogram of request sizes for requests.",
			Buckets: []float64{200, 500, 900, 1500},
		},
		[]string{},
	)

	// Create the handlers that will be wrapped by the middleware.
	pushHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Push"))
	})
	pullHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Pull"))
	})

	// Register all of the metrics in the standard registry.
	prometheus.MustRegister(inFlightGauge, counter, pullVec, pushVec, responseSize)

	// Wrap the pushHandler with our shared middleware, but use the
	// endpoint-specific pushVec with InstrumentHandlerDuration.
	pushChain := InstrumentHandlerInFlight(inFlightGauge,
		InstrumentHandlerCounter(counter,
			InstrumentHandlerDuration(pushVec,
				InstrumentHandlerResponseSize(responseSize, pushHandler),
			),
		),
	)

	// Wrap the pushHandler with the shared middleware, but use the
	// endpoint-specific pullVec with InstrumentHandlerDuration.
	pullChain := InstrumentHandlerInFlight(inFlightGauge,
		InstrumentHandlerCounter(counter,
			InstrumentHandlerDuration(pullVec,
				InstrumentHandlerResponseSize(responseSize, pullHandler),
			),
		),
	)

	http.Handle("/metrics", Handler())
	http.Handle("/push", pushChain)
	http.Handle("/pull", pullChain)

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatal(err)
	}
}
