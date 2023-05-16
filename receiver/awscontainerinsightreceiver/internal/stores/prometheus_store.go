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

package stores

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"go.uber.org/zap"
)

var (
	errMetricNameNotFound = errors.New("metricName not found from labels")
)

type PrometheusStore struct {
	logger   *zap.Logger
	appender appender
}

type appender struct {
	logger  *zap.Logger
	storage map[string]float64
}

func NewPrometheusStore(logger *zap.Logger, storage *map[string]float64) storage.Appendable {
	return &PrometheusStore{
		appender: appender{
			logger:  logger,
			storage: *storage,
		},
		logger: logger,
	}
}

func (a PrometheusStore) Appender(ctx context.Context) storage.Appender {
	return a.appender
}

func (a appender) Append(ref storage.SeriesRef, labels labels.Labels, atMs int64, value float64) (storage.SeriesRef, error) {
	metricName := labels.Get(model.MetricNameLabel)
	if metricName == "" {
		return 0, errMetricNameNotFound
	}

	a.logger.Debug(fmt.Sprintf("Scraped Metric name: %s, timestamp: %d, value: %f",
		metricName, atMs, value))

	for _, label := range labels {
		a.logger.Debug(fmt.Sprintf(
			"	->Label: %s, value: %s", label.Name, label.Value))
	}

	// TODO: update this to a reasonable storage mechanism
	a.storage[temporaryString(labels)] = value

	return 0, nil
}

func (a appender) Commit() error {
	// not necessary for our use-case
	return nil
}

func (a appender) Rollback() error {
	// not necessary for our use-case
	return nil
}

func (a appender) AppendExemplar(ref storage.SeriesRef, labels labels.Labels, exemplar exemplar.Exemplar) (storage.SeriesRef, error) {
	// not necessary for our use-case
	return 0, nil
}

func (a appender) AppendHistogram(ref storage.SeriesRef, labels labels.Labels, atMs int64, histogram *histogram.Histogram, floatHistogram *histogram.FloatHistogram) (storage.SeriesRef, error) {
	// not necessary for our use-case
	return 0, nil
}

func (a appender) UpdateMetadata(ref storage.SeriesRef, labels labels.Labels, metadata metadata.Metadata) (storage.SeriesRef, error) {
	// not necessary for our use-case
	return 0, nil
}

// TODO: this is temporary and is strictly used for testing the POC
func temporaryString(ls labels.Labels) string {
	var b bytes.Buffer

	b.WriteByte('{')
	for i, l := range ls {
		if l.Name == "instance" {
			continue
		}
		if i > 0 {
			b.WriteByte(',')
			b.WriteByte(' ')
		}
		b.WriteString(l.Name)
		b.WriteByte('=')
		b.WriteString(strconv.Quote(l.Value))
	}
	b.WriteByte('}')
	return b.String()
}
