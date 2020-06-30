# NOTE: Routes are evaluated in order. The first route to match will stop
# processing.

kind = "service-router"
name = "payments"
routes = [
  {
    match {
      http {
        path_prefix = "/currency"
      }
    }

    destination {
      service = "currency"
    }
  },
  {
    match {
      http {
        path_prefix = "/"
      }
    }

    destination {
      service = "payments"
    }
  },
]
