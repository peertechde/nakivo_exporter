package collector

import (
	"context"
	"strconv"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/peertechde/go-nakivo"
)

const (
	namespace = "nakivo"
)

var (
	defaultJobLabels = []string{"id"}
)

func NewJob(logger log.Logger, client *nakivo.Client, id int) *Job {
	return &Job{
		logger: logger,
		client: client,
		id:     id,

		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: prometheus.BuildFQName(namespace, "job_stats", "up"),
			Help: "Was the last scrape of the Nakivo job endpoint successful.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: prometheus.BuildFQName(namespace, "job_stats", "total_scrapes"),
			Help: "Current total Nakivo job scrapes.",
		}),

		metrics: []jobMetric{
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "job", "last_recent_status"),
					"The status of the last job run.",
					defaultJobLabels, nil,
				),
				value: func(resp jobInfoResponse) float64 {
					switch resp.lastRecentStatus {
					case "OK", "WAITING_DEMAND", "WAITING_SCHEDULE", "RUNNING":
						return 1
					case "FAILED", "STOPPED":
						fallthrough
					default:
						return 0
					}
				},
			},
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "job", "last_recent_speed"),
					"The speed of the last job run.",
					defaultJobLabels, nil,
				),
				value: func(resp jobInfoResponse) float64 {
					return float64(resp.lastRecentSpeed)
				},
			},
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "job", "last_recent_duration_ms"),
					"The duration of the last job run.",
					defaultJobLabels, nil,
				),
				value: func(resp jobInfoResponse) float64 {
					return float64(resp.lastRecentDurationMs)
				},
			},
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "job", "last_recent_data_kb"),
					"The amount of data transferred during the last job run.",
					defaultJobLabels, nil,
				),
				value: func(resp jobInfoResponse) float64 {
					return float64(resp.lastRecentDataKb)
				},
			},
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "job", "last_recent_vms_ok"),
					"The amount of virtual machines successfully processed during the last job run.",
					defaultJobLabels, nil,
				),
				value: func(resp jobInfoResponse) float64 {
					return float64(resp.lastRecentVMsOK)
				},
			},
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "job", "last_recent_vms_failed"),
					"The amount of virtual machines failed during the last job run.",
					defaultJobLabels, nil,
				),
				value: func(resp jobInfoResponse) float64 {
					return float64(resp.lastRecentVMsFailed)
				},
			},
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "job", "last_recent_vms_stopped"),
					"The amount of virtual machines stopped during the last job run.",
					defaultJobLabels, nil,
				),
				value: func(resp jobInfoResponse) float64 {
					return float64(resp.lastRecentVMsStopped)
				},
			},
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "job", "last_recent_compression_ratio"),
					"The compression ratio during the last job run.",
					defaultJobLabels, nil,
				),
				value: func(resp jobInfoResponse) float64 {
					return float64(resp.lastRecentCompressionRatio)
				},
			},
		},
	}
}

type jobInfoResponse struct {
	vmCount                    int
	diskCount                  int
	sourcesSize                int64
	lastRecentStatus           string
	lastRecentSpeed            int64
	lastRecentDurationMs       int64
	lastRecentDataKb           int64
	lastRecentVMsOK            int
	lastRecentVMsFailed        int
	lastRecentVMsStopped       int
	lastRecentCompressionRatio int
}

type jobMetric struct {
	metricType prometheus.ValueType
	desc       *prometheus.Desc
	value      func(resp jobInfoResponse) float64
}

type Job struct {
	logger log.Logger
	client *nakivo.Client

	// name of the job
	name string

	// id of the job
	id int

	up           prometheus.Gauge
	totalScrapes prometheus.Counter

	metrics []jobMetric
}

func (j *Job) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range j.metrics {
		ch <- metric.desc
	}
	ch <- j.up.Desc()
	ch <- j.totalScrapes.Desc()
}

func (j *Job) Collect(ch chan<- prometheus.Metric) {
	j.totalScrapes.Inc()

	if err := j.scrape(ch); err != nil {
		j.up.Set(0)
	} else {
		j.up.Set(1)
	}

	ch <- j.up
	ch <- j.totalScrapes
}

func (j *Job) scrape(ch chan<- prometheus.Metric) error {
	ctx := context.Background()
	jobs, _, err := j.client.Job.JobInfo(ctx, []int{j.id}, 0)
	if err != nil {
		level.Warn(j.logger).Log("msg", "failed to fetch job info stat", "err", err)
		return err
	}
	if len(jobs.Children) == 0 || len(jobs.Children) >= 2 {
		level.Warn(j.logger).Log("msg", "failed to fetch job info stat")
		return err
	}

	stats := jobs.Children[0]
	resp := jobInfoResponse{
		vmCount:                    stats.VmCount,
		diskCount:                  stats.DiskCount,
		sourcesSize:                stats.SourcesSize,
		lastRecentStatus:           stats.LrState,
		lastRecentSpeed:            stats.LrSpeed,
		lastRecentDurationMs:       stats.LrDurationMs,
		lastRecentDataKb:           stats.LrDataKb,
		lastRecentVMsOK:            stats.LrVmOk,
		lastRecentVMsFailed:        stats.LrVmFailed,
		lastRecentVMsStopped:       stats.LrVmStopped,
		lastRecentCompressionRatio: stats.LrCompressionRatio,
	}
	for _, metric := range j.metrics {
		ch <- prometheus.MustNewConstMetric(
			metric.desc,
			metric.metricType,
			metric.value(resp),
			strconv.Itoa(j.id),
		)
	}

	return nil
}
