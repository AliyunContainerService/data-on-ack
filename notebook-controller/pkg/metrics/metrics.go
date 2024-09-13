/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
*limitations under the License.
*/
    
package metrics

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

type Metrics struct {
	cli                      client.Client
	runningNotebooks         *prometheus.GaugeVec
	NotebookCreation         *prometheus.CounterVec
	NotebookFailCreation     *prometheus.CounterVec
	NotebookCullingCount     *prometheus.CounterVec
	NotebookCullingTimestamp *prometheus.GaugeVec
}

func NewMetrics(cli client.Client) *Metrics {
	m := &Metrics{
		cli: cli,
		runningNotebooks: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "notebook_running",
				Help: "Current running notebooks in the cluster",
			},
			[]string{"namespace"},
		),
		NotebookCreation: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notebook_create_total",
				Help: "Total times of creating notebooks",
			},
			[]string{"namespace"},
		),
		NotebookFailCreation: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notebook_create_failed_total",
				Help: "Total failure times of creating notebooks",
			},
			[]string{"namespace"},
		),
		NotebookCullingCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "notebook_culling_total",
				Help: "Total times of culling notebooks",
			},
			[]string{"namespace", "name"},
		),
		NotebookCullingTimestamp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "last_notebook_culling_timestamp_seconds",
				Help: "Timestamp of the last notebook culling in seconds",
			},
			[]string{"namespace", "name"},
		),
	}

	metrics.Registry.MustRegister(m)
	return m
}

func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	m.runningNotebooks.Describe(ch)
	m.NotebookCreation.Describe(ch)
	m.NotebookFailCreation.Describe(ch)
}

func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	m.scrape()
	m.runningNotebooks.Collect(ch)
	m.NotebookCreation.Collect(ch)
	m.NotebookFailCreation.Collect(ch)
}

func (m *Metrics) scrape() {
	stsList := &appsv1.StatefulSetList{}
	err := m.cli.List(context.TODO(), stsList)
	if err != nil {
		return
	}
	stsCache := make(map[string]float64)
	for _, v := range stsList.Items {
		name, ok := v.Spec.Template.GetLabels()["notebook-name"]
		if ok && name == v.Name {
			stsCache[v.Namespace] += 1
		}
	}

	for ns, v := range stsCache {
		m.runningNotebooks.WithLabelValues(ns).Set(v)
	}
}
