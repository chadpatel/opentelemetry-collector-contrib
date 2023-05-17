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

package k8sapiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8sapiserver"

import (
	"context"
	"time"

	gokitLog "github.com/go-kit/log"
	httpconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/logging"
)

const (
	collectionEndpoint = "/metrics"
	bearerTokenPath    = "/var/run/secrets/kubernetes.io/serviceaccount/token" // #nosec
	collectionInterval = 60 * time.Second                                      // TODO: configure via YAML?
	collectionTimeout  = 30 * time.Second                                      // TODO: configure via YAML?
)

type MetricScrape struct {
	scrapeManager    *scrape.Manager
	discoveryManager *discovery.Manager
	logger           *zap.Logger
	gokitlogger      gokitLog.Logger
	store            *storage.Appendable
	config           *config.Config
	cancel           context.CancelFunc
	context          context.Context
}

func NewMetricScrape(host string, logger *zap.Logger, store *storage.Appendable) (*MetricScrape, error) {
	// TODO
	/* 2023-05-16T18:00:47.956Z        debug   scrape/scrape.go:1351   Scrape failed   {"kind": "receiver", "name": "awscontainerinsightreceiver", "data_type": "metrics", "scrape_pool": "k8sapiserver", "target": "https://ip-192-168-71-146.us-east-2.compute.internal:443/metrics", "error": "Get \"https://ip-192-168-71-146.us-east-2.compute.internal:443/metrics\": unable to read authorization credentials file /var/Run/secrets/kubernetes.io/serviceaccount/token: open /var/Run/secrets/kubernetes.io/serviceaccount/token: no such file or directory"}
	 */

	config := &config.Config{
		ScrapeConfigs: []*config.ScrapeConfig{
			{
				JobName: "k8sapiserver",

				ScrapeInterval: model.Duration(collectionInterval),
				ScrapeTimeout:  model.Duration(collectionTimeout),

				HonorLabels:     true,
				HonorTimestamps: true,

				MetricsPath: collectionEndpoint,
				Scheme:      "https",

				HTTPClientConfig: httpconfig.HTTPClientConfig{
					TLSConfig: httpconfig.TLSConfig{
						CAFile: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
					},
					BearerTokenFile: bearerTokenPath,
					FollowRedirects: true,
				},

				ServiceDiscoveryConfigs: discovery.Configs{
					discovery.StaticConfig{
						&targetgroup.Group{
							Targets: []model.LabelSet{
								{model.AddressLabel: model.LabelValue(host)},
							},
							Source: "0",
						},
					},
				},
			},
		},
	}

	// the prometheus scraper library uses gokit logging, we must adapt our zap logger to gokit
	gokitlogger := logging.NewZapToGokitLogAdapter(logger)

	ms := MetricScrape{
		logger:      logger,
		gokitlogger: gokitlogger,
		store:       store,
		config:      config,
	}

	ms.context, ms.cancel = context.WithCancel(context.Background())

	ms.scrapeManager = scrape.NewManager(&scrape.Options{PassMetadataInContext: true}, gokitlogger, *store)
	ms.discoveryManager = discovery.NewManager(ms.context, gokitlogger)

	return &ms, nil
}

func (ms *MetricScrape) Run() error {
	err := ms.scrapeManager.ApplyConfig(ms.config)
	if err != nil {
		return err
	}

	discoveryConfigs := make(map[string]discovery.Configs)
	discoveryConfigs["k8sapiserver"] = ms.config.ScrapeConfigs[0].ServiceDiscoveryConfigs
	err = ms.discoveryManager.ApplyConfig(discoveryConfigs)
	if err != nil {
		return err
	}

	go ms.runDiscoveryManager()
	go ms.runScrapeManager()

	return nil
}

func (ms *MetricScrape) Shutdown(context.Context) error {
	if ms.cancel != nil {
		ms.cancel()
	}
	if ms.scrapeManager != nil {
		ms.scrapeManager.Stop()
	}
	return nil
}

func (ms *MetricScrape) runDiscoveryManager() {
	ms.logger.Info("Starting k8sapiserver prometheus metrics discovery manager")
	if err := ms.discoveryManager.Run(); err != nil {
		ms.logger.Error("Discovery manager failed", zap.Error(err))
		return
	}

	select {
	case <-ms.context.Done():
		ms.logger.Info("Stopping k8sapiserver prometheus metrics discovery manager")
		return
	default:
	}
}

func (ms *MetricScrape) runScrapeManager() {
	// The scrape manager needs to wait for the configuration to be loaded before beginning
	// TODO <-r.configLoaded
	// TODO: is there anything to do here?
	ms.logger.Warn("Starting k8sapiserver prometheus metrics scrape manager")
	if err := ms.scrapeManager.Run(ms.discoveryManager.SyncCh()); err != nil {
		ms.logger.Error("Scrape manager failed", zap.Error(err))
		return
	}

	select {
	case <-ms.context.Done():
		ms.logger.Info("Stopping k8sapiserver prometheus metrics scrape manager")
		return
	default:
	}
}
