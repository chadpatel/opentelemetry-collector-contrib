// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sapiserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

func TestNewMetricScrape(t *testing.T) {
	logger := zap.NewNop()
	storage := map[string]float64{}
	store := stores.NewPrometheusStore(logger, &storage)

	ms, err := NewMetricScrape("host_name", logger, &store)
	assert.NoError(t, err)
	assert.NotNil(t, ms)
}

func TestMetricScrapeRun(t *testing.T) {
	logger := zap.NewNop()
	storage := map[string]float64{}
	store := stores.NewPrometheusStore(logger, &storage)

	var renameMetric = `
# HELP http_go_threads Number of OS threads created
# TYPE http_go_threads gauge
http_go_threads 19

# HELP http_connected_total connected clients
# TYPE http_connected_total counter
http_connected_total{method="post",port="6380"} 15.0

# HELP redis_http_requests_total Redis connected clients
# TYPE redis_http_requests_total counter
redis_http_requests_total{method="post",port="6380"} 10.0
redis_http_requests_total{method="post",port="6381"} 12.0

# HELP rpc_duration_total RPC clients
# TYPE rpc_duration_total counter
rpc_duration_total{method="post",port="6380"} 100.0
rpc_duration_total{method="post",port="6381"} 120.0
`

	targets := []*testData{
		{
			name: "k8sapiserver",
			pages: []mockPrometheusResponse{
				{code: 200, data: renameMetric},
			},
		},
	}
	mp, cfg, err := setupMockPrometheus(targets...)
	assert.NoError(t, err)
	assert.NotNil(t, mp)
	defer mp.Close()

	ms, err := NewMetricScrape(mp.srv.URL, logger, &store)
	ms.config = cfg // use test config
	assert.NoError(t, err)
	assert.NotNil(t, ms)

	assert.NoError(t, ms.Run())

	t.Cleanup(func() {
		assert.Len(t, ms.scrapeManager.TargetsActive(), len(targets))
		assert.NoError(t, ms.Shutdown(ms.context))
		assert.Len(t, flattenTargets(ms.scrapeManager.TargetsActive()), 0)
	})

	// wait for multiple scrapes
	for i := 0; i < 2; i++ {
		mp.wg.Wait()
	}

	// metrics
	assert.Equal(t, float64(10), storage["{__name__=\"redis_http_requests_total\", job=\"k8sapiserver\", method=\"post\", port=\"6380\"}"])

	// metadata
	assert.Equal(t, float64(6), storage["{__name__=\"scrape_samples_scraped\", job=\"k8sapiserver\"}"])
}
