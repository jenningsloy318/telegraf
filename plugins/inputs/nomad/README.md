# Hashicorp Nomad Input Plugin

This plugin collects metrics from every [Nomad agent][nomad] of the specified
cluster. Telegraf may be present in every node and connect to the agent locally.

⭐ Telegraf v1.22.0
🏷️ server
💻 all

[nomad]: https://www.nomadproject.io/

## Global configuration options <!-- @/docs/includes/plugin_config.md -->

In addition to the plugin-specific configuration settings, plugins support
additional global and plugin configuration settings. These settings are used to
modify metrics, tags, and field or create aliases and configure ordering, etc.
See the [CONFIGURATION.md][CONFIGURATION.md] for more details.

[CONFIGURATION.md]: ../../../docs/CONFIGURATION.md#plugins

## Configuration

```toml @sample.conf
# Read metrics from the Nomad API
[[inputs.nomad]]
  ## URL for the Nomad agent
  # url = "http://127.0.0.1:4646"

  ## Set response_timeout (default 5 seconds)
  # response_timeout = "5s"

  ## Optional TLS Config
  # tls_ca = /path/to/cafile
  # tls_cert = /path/to/certfile
  # tls_key = /path/to/keyfile
```

## Metrics

Both Nomad servers and agents collect various metrics. For every details, please
have a look at [Nomad metrics][metrics] and [Nomad telemetry][telemetry]
ocumentation.

[metrics]: https://www.nomadproject.io/docs/operations/metrics
[telemetry]: https://www.nomadproject.io/docs/operations/telemetry

## Example Output

There is no predefined metric format, so output depends on plugin input.
