package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const metricsNamespace = "service_launchpad_control_plane"

type controlPlaneMetrics struct {
	mu sync.RWMutex

	store *serviceStore

	serviceRegistrationsByResult map[string]uint64
	deploymentsByResult          map[string]uint64
	deploymentDurationBuckets    map[string][]uint64
	deploymentDurationCount      map[string]uint64
	deploymentDurationSum        map[string]float64
}

var deploymentDurationBuckets = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}

func newControlPlaneMetrics(store *serviceStore) *controlPlaneMetrics {
	return &controlPlaneMetrics{
		store:                        store,
		serviceRegistrationsByResult: map[string]uint64{"success": 0, "failure": 0},
		deploymentsByResult:          map[string]uint64{"success": 0, "failure": 0},
		deploymentDurationBuckets: map[string][]uint64{
			"success": make([]uint64, len(deploymentDurationBuckets)),
			"failure": make([]uint64, len(deploymentDurationBuckets)),
		},
		deploymentDurationCount: map[string]uint64{"success": 0, "failure": 0},
		deploymentDurationSum:   map[string]float64{"success": 0, "failure": 0},
	}
}

func (m *controlPlaneMetrics) recordServiceRegistration(result string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.serviceRegistrationsByResult[result]++
}

func (m *controlPlaneMetrics) recordDeployment(result string, duration time.Duration) {
	seconds := duration.Seconds()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.deploymentsByResult[result]++
	m.deploymentDurationCount[result]++
	m.deploymentDurationSum[result] += seconds

	for i, bucket := range deploymentDurationBuckets {
		if seconds <= bucket {
			m.deploymentDurationBuckets[result][i]++
		}
	}
}

func (m *controlPlaneMetrics) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(m.render()))
}

func (m *controlPlaneMetrics) render() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var b strings.Builder

	writeMetricHelp(&b, "service_registrations_total", "Service registration attempts by result.")
	writeMetricType(&b, "service_registrations_total", "counter")
	writeCounterWithResult(&b, "service_registrations_total", "success", m.serviceRegistrationsByResult["success"])
	writeCounterWithResult(&b, "service_registrations_total", "failure", m.serviceRegistrationsByResult["failure"])

	writeMetricHelp(&b, "deployments_total", "Service deployment attempts by result.")
	writeMetricType(&b, "deployments_total", "counter")
	writeCounterWithResult(&b, "deployments_total", "success", m.deploymentsByResult["success"])
	writeCounterWithResult(&b, "deployments_total", "failure", m.deploymentsByResult["failure"])

	writeMetricHelp(&b, "deployment_duration_seconds", "Duration of service deployment attempts.")
	writeMetricType(&b, "deployment_duration_seconds", "histogram")
	writeHistogram(&b, "success", m.deploymentDurationBuckets["success"], m.deploymentDurationCount["success"], m.deploymentDurationSum["success"])
	writeHistogram(&b, "failure", m.deploymentDurationBuckets["failure"], m.deploymentDurationCount["failure"], m.deploymentDurationSum["failure"])

	writeMetricHelp(&b, "managed_services", "Current number of services registered in the control plane.")
	writeMetricType(&b, "managed_services", "gauge")
	fmt.Fprintf(&b, "%s_managed_services %d\n", metricsNamespace, m.store.count())

	return b.String()
}

func writeMetricHelp(b *strings.Builder, name, help string) {
	fmt.Fprintf(b, "# HELP %s_%s %s\n", metricsNamespace, name, help)
}

func writeMetricType(b *strings.Builder, name, metricType string) {
	fmt.Fprintf(b, "# TYPE %s_%s %s\n", metricsNamespace, name, metricType)
}

func writeCounterWithResult(b *strings.Builder, name, result string, value uint64) {
	fmt.Fprintf(b, "%s_%s{result=%q} %d\n", metricsNamespace, name, result, value)
}

func writeHistogram(b *strings.Builder, result string, bucketCounts []uint64, count uint64, sum float64) {
	metricName := metricsNamespace + "_deployment_duration_seconds"
	for i, bucket := range deploymentDurationBuckets {
		fmt.Fprintf(b, "%s_bucket{result=%q,le=%q} %d\n", metricName, result, strconv.FormatFloat(bucket, 'f', -1, 64), bucketCounts[i])
	}
	fmt.Fprintf(b, "%s_bucket{result=%q,le=\"+Inf\"} %d\n", metricName, result, count)
	fmt.Fprintf(b, "%s_sum{result=%q} %g\n", metricName, result, sum)
	fmt.Fprintf(b, "%s_count{result=%q} %d\n", metricName, result, count)
}
