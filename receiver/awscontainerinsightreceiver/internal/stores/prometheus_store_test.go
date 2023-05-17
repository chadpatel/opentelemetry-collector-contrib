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
	"context"
	"sync"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestPrometheusStore(t *testing.T) {
	storage := map[string]float64{}
	lock := sync.Mutex{}
	store := NewPrometheusStore(zap.NewNop(), &storage, &lock)
	ctx := context.TODO()

	myAppender := store.Appender(ctx)
	assert.NotNil(t, myAppender)

	response, err := myAppender.Append(0, labels.FromStrings(model.MetricNameLabel, "myLabel"), 0, 1234.5)
	assert.NoError(t, err)
	assert.Zero(t, response)

	lock.Lock()
	defer lock.Unlock()
	assert.Equal(t, 1234.5, storage["{__name__=\"myLabel\"}"])
}

func TestAppendNoMetricName(t *testing.T) {
	storage := map[string]float64{}
	store := NewPrometheusStore(zap.NewNop(), &storage, &sync.Mutex{})
	ctx := context.TODO()

	myAppender := store.Appender(ctx)
	assert.NotNil(t, myAppender)

	response, err := myAppender.Append(0, labels.FromStrings("not_metric_name", "myLabel"), 0, 1234.5)
	assert.Error(t, err)
	assert.Zero(t, response)
}

func TestNoOpMethods(t *testing.T) {
	storage := map[string]float64{}
	store := NewPrometheusStore(zap.NewNop(), &storage, &sync.Mutex{})
	ctx := context.TODO()

	myAppender := store.Appender(ctx)
	assert.NotNil(t, myAppender)

	err := myAppender.Commit()
	assert.NoError(t, err)

	err = myAppender.Rollback()
	assert.NoError(t, err)

	myLabel := labels.FromStrings(model.MetricNameLabel, "myLabel")
	myExemplar := exemplar.Exemplar{}
	response, err := myAppender.AppendExemplar(0, myLabel, myExemplar)
	assert.NoError(t, err)
	assert.Zero(t, response)
	assert.Zero(t, len(storage))

	myHistogram := histogram.Histogram{}
	myFloatHistogram := histogram.FloatHistogram{}
	response, err = myAppender.AppendHistogram(0, myLabel, 0, &myHistogram, &myFloatHistogram)
	assert.NoError(t, err)
	assert.Zero(t, response)
	assert.Zero(t, len(storage))

	myMetadata := metadata.Metadata{}
	response, err = myAppender.UpdateMetadata(0, myLabel, myMetadata)
	assert.NoError(t, err)
	assert.Zero(t, response)
	assert.Zero(t, len(storage))
}
