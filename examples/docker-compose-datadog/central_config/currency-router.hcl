kind = "service-router"
name = "currency"
routes = [
  {
    match {
      http {
        path_prefix = "/"
      }
    }

    destination {
      service               = "currency"
      num_retries           = 2
      retry_on_status_codes = [500]
    }
  },
]
