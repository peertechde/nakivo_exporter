package collector

import (
	"context"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/peertechde/go-nakivo"
)

func NewJobGroup(logger log.Logger, client *nakivo.Client) *JobGroup {
	return &JobGroup{
		logger: logger,
		client: client,

		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: prometheus.BuildFQName(namespace, "group_stats", "up"),
			Help: "Was the last scrape of the Nakivo group endpoint successful.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: prometheus.BuildFQName(namespace, "group_stats", "total_scrapes"),
			Help: "Current total Nakivo group scrapes.",
		}),

		metrics: []groupMetric{
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "group", "jobs_total"),
					"The number of enabled jobs.",
					nil, nil,
				),
				value: func(resp jobGroupResponse) float64 {
					return float64(resp.jobCount)
				},
			},
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "group", "vms_total"),
					"The number virtual machines processed by the jobs inthe the group.",
					nil, nil,
				),
				value: func(resp jobGroupResponse) float64 {
					return float64(resp.vmCount)
				},
			},
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "group", "disks_total"),
					"The number of disks processed by the jobs inthe the group.",
					nil, nil,
				),
				value: func(resp jobGroupResponse) float64 {
					return float64(resp.diskCount)
				},
			},
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "group", "last_recent_jobs_ok"),
					"The number successful jobs during the last run.",
					nil, nil,
				),
				value: func(resp jobGroupResponse) float64 {
					return float64(resp.lastRecentOK)
				},
			},
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "group", "last_recent_jobs_failed"),
					"The number failed jobs during the last run.",
					nil, nil,
				),
				value: func(resp jobGroupResponse) float64 {
					return float64(resp.lastRecentFailed)
				},
			},
			{
				metricType: prometheus.GaugeValue,
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "group", "last_recent_jobs_stopped"),
					"The number stopped jobs during the last run.",
					nil, nil,
				),
				value: func(resp jobGroupResponse) float64 {
					return float64(resp.lastRecentStopped)
				},
			},
		},
	}
}

type jobGroupResponse struct {
	jobCount          int
	vmCount           int
	diskCount         int
	sourcesSize       int // TODO:
	lastRecentOK      int
	lastRecentFailed  int
	lastRecentStopped int
}

type groupMetric struct {
	metricType prometheus.ValueType
	desc       *prometheus.Desc
	value      func(resp jobGroupResponse) float64
}

type JobGroup struct {
	logger log.Logger
	client *nakivo.Client

	up           prometheus.Gauge
	totalScrapes prometheus.Counter

	metrics []groupMetric
}

func (g *JobGroup) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range g.metrics {
		ch <- metric.desc
	}
	ch <- g.up.Desc()
	ch <- g.totalScrapes.Desc()
}

func (g *JobGroup) Collect(ch chan<- prometheus.Metric) {
	g.totalScrapes.Inc()

	if err := g.scrape(ch); err != nil {
		g.up.Set(0)
	} else {
		g.up.Set(1)
	}

	ch <- g.up
	ch <- g.totalScrapes
}

func (g *JobGroup) scrape(ch chan<- prometheus.Metric) error {
	ctx := context.Background()
	group, _, err := g.client.Job.List(ctx, 0, false)
	if err != nil {
		level.Warn(g.logger).Log("msg", "failed to fetch group stat", "err", err)
		return err
	}
	if len(group.Children) == 0 || len(group.Children) >= 2 {
		level.Warn(g.logger).Log("msg", "failed to fetch group stat")
		return err
	}

	stats := group.Children[0]
	resp := jobGroupResponse{
		jobCount:          stats.JobCountEnabled,
		vmCount:           stats.VMCount,
		diskCount:         stats.DiskCount,
		lastRecentOK:      stats.LrJobOk,
		lastRecentFailed:  stats.LrJobFailed,
		lastRecentStopped: stats.LrJobStopped,
	}
	for _, metric := range g.metrics {
		ch <- prometheus.MustNewConstMetric(
			metric.desc,
			metric.metricType,
			metric.value(resp),
		)
	}

	return nil
}
