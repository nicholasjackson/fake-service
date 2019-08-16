package handlers

import (
	"fmt"
	"net/http"
)

func healthHandler(rw http.ResponseWriter, r *http.Request) {
	logger.Info("Handling health request")

	fmt.Fprint(rw, "OK")
}
