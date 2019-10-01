data_dir = "/tmp/"
log_level = "DEBUG"

datacenter = "dc1"

server = true

bootstrap_expect = 1
ui = true

bind_addr = "0.0.0.0"
client_addr = "0.0.0.0"

ports {
  grpc = 8502
}

connect {
  enabled = true
}

telemetry {
  dogstatsd_addr = "10.5.0.7:8125"
}

advertise_addr = "10.5.0.2"
enable_central_service_config = true

config_entries {
  bootstrap = [
    {
      kind = "proxy-defaults"
      name = "global"
      
      config {
        envoy_dogstatsd_url = "udp://10.5.0.8:8125"
      
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
                                            "address": "10.5.0.8",
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
                  "name": "envoy.tracers.datadog",
                  "config": {
                      "collector_cluster": "datadog_8126",
                      "service_name": "envoy"
                  }
              }
          }
        EOL
      }
    }
  ]
}
