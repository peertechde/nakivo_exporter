package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"os"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"golang.org/x/net/publicsuffix"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/peertechde/go-nakivo"
	"github.com/peertechde/nakivo_exporter/collector"
)

var (
	listenAddress = kingpin.Flag("web.listen-address",
		"Address to listen on for web interface and telemetry.").
		Default(":9777").String()
	metricsPath = kingpin.Flag("web.telemetry-path",
		"Path under which to expose metrics.").
		Default("/metrics").String()
	tlsInsecureSkipVerify = kingpin.Flag(
		"tls.insecure-skip-verify",
		"Ignore certificate and server verification when using a tls connection.").
		Bool()

	nakivoAddress = kingpin.Flag("nakivo.addr",
		"HTTP API address of the nakivo endpoint.").
		Default("https://localhost:4443/c/router").String()
	nakivoPort = kingpin.Flag("nakivo.port",
		"HTTP API port of the nakivo endpoint.").
		Default("4443").Int()
	nakivoUser = kingpin.Flag("nakivo.user",
		"The nakivo user.").
		Default("admin").String()
	nakivoPassword = kingpin.Flag("nakivo.password",
		"The nakivo user password.").
		String()
	nakivoTimeout = kingpin.Flag("nakivo.timeout",
		"Timeout for trying to get stats from Nakivo.").
		Default("5s").Duration()
)

func main() {
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("nakivo_exporter"))
	kingpin.CommandLine.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		level.Error(logger).Log("msg", "failed to create new cookiejar", "err", err)
		os.Exit(1)
	}
	httpClient := &http.Client{
		Timeout: *nakivoTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			Proxy: http.ProxyFromEnvironment,
		},
		Jar: jar,
	}
	nakivoClient, err := nakivo.NewClient(httpClient, *nakivoAddress, *nakivoPort)
	if err != nil {
		level.Error(logger).Log("msg", "failed to create nakivo client", "err", err)
		os.Exit(1)
	}
	_, _, err = nakivoClient.Authentication.Login(context.Background(), *nakivoUser, *nakivoPassword, false)
	if err != nil {
		level.Error(logger).Log("msg", "failed to parse login", "err", err)
		os.Exit(1)
	}

	prometheus.MustRegister(version.NewCollector("nakivo_exporter"))
	prometheus.MustRegister(collector.NewJob(logger, nakivoClient, 9))
	prometheus.MustRegister(collector.NewJobGroup(logger, nakivoClient))

	level.Info(logger).Log("msg", "Starting nakivo_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "build_context", version.BuildContext())

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Node Exporter</title></head>
			<body>
			<h1>Nakivo Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})

	level.Info(logger).Log("msg", "Listening on", "address", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
