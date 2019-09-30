service {
  name = "cache"
  id = "cache-v1"
  address = "10.5.0.5"
  port = 9090
  
  connect { 
    sidecar_service {
      port = 20000
      
      check {
        name = "Connect Envoy Sidecar"
        tcp = "10.5.0.5:20000"
        interval ="10s"
      }
    }  
  }
}
