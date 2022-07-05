package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/nicholasjackson/fake-service/errors"
	"github.com/nicholasjackson/fake-service/logging"
)

type Config struct {
	logger        *logging.Logger
	errorInjector *errors.Injector
	healthHandler *Health
}

// NewHealth creates a new health handler
func NewConfig(logger *logging.Logger, ej *errors.Injector, hh *Health) *Config {
	return &Config{
		logger,
		ej,
		hh,
	}
}

// Handle the request
func (c *Config) Handle(rw http.ResponseWriter, r *http.Request) {
	c.logger.Log().Info("Config called", "path", r.URL.Path)

	// get the parameters from the path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(rw, "Config endpoint expects /config/parameter/value", http.StatusBadRequest)
		return
	}

	c.logger.Log().Info("Set config", "parameter", parts[2], "value", parts[3])

	switch parts[2] {
	case "error_rate":
		rate, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			c.logger.Log().Error("Config endpoint error_rate expects a floating point value between 0 and 1", "data", parts, "error", err)

			http.Error(rw, "Config endpoint error_rate expects a floating point value between 0 and 1", http.StatusBadRequest)
			return
		}

		c.errorInjector.SetErrorPercentage(rate)
	case "health_check_response_code":
		code, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			c.logger.Log().Error("Config endpoint health_check_response_code expects an int value representing an HTTP status code", "data", parts, "error", err)

			http.Error(rw, "Config endpoint health_check_response_code expects an int value representing an HTTP status code", http.StatusBadRequest)
			return
		}

		c.healthHandler.SetStatusCode(int(code))
	default:
		http.Error(rw, "Invalid parameter", http.StatusBadRequest)
		return
	}

	rw.WriteHeader(http.StatusOK)
}
