# Nakivo Exporter

Prometheus exporter for metrics about Nakivo, written in Go.

## Usage

| Argument                 | Description | Default     |
| ------------------------ | ----------- | ----------- |
| web.listen-address       | Address to listen on for web interface and telemetry. | :9777 |
| web.telemetry-path       | Path under which to expose metrics. | /metrics |
| tls.insecure-skip-verify | Ignore certificate and server verification when using a tls connection. | |
| nakivo.addr              | IP address of the nakivo endpoint. | 127.0.0.1|
| nakivo.port              | Port of the nakivo endpoint | 4443 |
| nakivo.user              | The nakivo user. | admin |
| nakivo.password          | The nakivo user password. | |
| nakivo.timeout           | Timeout for trying to get stats from Nakivo. | 5s |
| version                  | Show application version. | |
