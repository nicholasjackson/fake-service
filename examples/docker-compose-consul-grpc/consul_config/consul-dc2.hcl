data_dir = "/tmp/"
log_level = "DEBUG"

datacenter = "dc2"
primary_datacenter = "dc1"

server = true

bootstrap_expect = 1

bind_addr = "0.0.0.0"
client_addr = "0.0.0.0"

ports {
  grpc = 8502
}

connect {
  enabled = true
}

ui = true
enable_central_service_config = true

advertise_addr = "10.6.0.2"
advertise_addr_wan = "192.169.7.4"
retry_join_wan = ["192.169.7.2"]
