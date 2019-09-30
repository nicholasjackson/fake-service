service {
  name = "currency"
  id = "currency-v1"
  address = "10.5.0.7"
  port = 9090
  
  connect { 
    sidecar_service {
      port = 20000
      
      check {
        name = "Connect Envoy Sidecar"
        tcp = "10.5.0.7:20000"
        interval ="10s"
      }
    }  
  }
}
