kind = "proxy-defaults"
name = "global"

config {
  envoy_dogstatsd_url = 10.5.0.7":8125"

  envoy_extra_static_clusters_json = <<EOL
    {
      "connect_timeout": "3.000s",
      "dns_lookup_family": "V4_ONLY",
      "lb_policy": "ROUND_ROBIN",
      "load_assignment": {
          "cluster_name": "datadog_8126",
          "endpoints": [
              {
                  "lb_endpoints": [
                      {
                          "endpoint": {
                              "address": {
                                  "socket_address": {
                                      "address": "10.5.0.7",
                                      "port_value": 8126,
                                      "protocol": "TCP"
                                  }
                              }
                          }
                      }
                  ]
              }
          ]
      },
      "name": "datadog_8126",
      "type": "STRICT_DNS"
    }
  EOL

  envoy_tracing_json = <<EOL
    {
        "http": {
            "config": {
                "collector_cluster": "datadog_8126",
            },
            "name": "envoy.tracers.datadog"
        }
    }
  EOL
}
