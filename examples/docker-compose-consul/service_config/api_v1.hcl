service {
  name = "api"
  id = "api-v1"
  port = 9090
 
  # Required in order to allow registration of a sidecar
  connect { 
    sidecar_service {
    }
  }
}
