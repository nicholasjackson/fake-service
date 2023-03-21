package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nicholasjackson/fake-service/client"
	"github.com/nicholasjackson/fake-service/errors"
	"github.com/nicholasjackson/fake-service/grpc/api"
	"github.com/nicholasjackson/fake-service/load"
	"github.com/nicholasjackson/fake-service/logging"
	"github.com/nicholasjackson/fake-service/response"
	"github.com/nicholasjackson/fake-service/timing"
	"github.com/nicholasjackson/fake-service/worker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FakeServer implements the gRPC interface
type FakeServer struct {
	api.UnimplementedFakeServiceServer
	name             string
	message          string
	duration         *timing.RequestDuration
	upstreamURIs     []string
	workerCount      int
	defaultClient    client.HTTP
	grpcClients      map[string]client.GRPC
	errorInjector    *errors.Injector
	loadGenerator    *load.Generator
	log              *logging.Logger
	requestGenerator load.RequestGenerator
	waitTillReady    bool
	readinessHandler *Ready
}

// NewFakeServer creates a new instance of FakeServer
func NewFakeServer(
	name, message string,
	duration *timing.RequestDuration,
	upstreamURIs []string,
	workerCount int,
	defaultClient client.HTTP,
	grpcClients map[string]client.GRPC,
	i *errors.Injector,
	loadGenerator *load.Generator,
	l *logging.Logger,
	requestGenerator load.RequestGenerator,
	waitTillReady bool,
	readinessHandler *Ready,
) *FakeServer {

	return &FakeServer{
		UnimplementedFakeServiceServer: api.UnimplementedFakeServiceServer{},
		name:                           name,
		message:                        message,
		duration:                       duration,
		upstreamURIs:                   upstreamURIs,
		workerCount:                    workerCount,
		defaultClient:                  defaultClient,
		grpcClients:                    grpcClients,
		errorInjector:                  i,
		loadGenerator:                  loadGenerator,
		log:                            l,
		requestGenerator:               requestGenerator,
		waitTillReady:                  waitTillReady,
		readinessHandler:               readinessHandler,
	}
}

// Handle implements the FakeServer Handle interface method
func (f *FakeServer) Handle(ctx context.Context, in *api.Request) (*api.Response, error) {
	if f.waitTillReady && !f.readinessHandler.Complete() {
		f.log.Log().Info("Service Unavailable")
		return nil, status.Error(codes.Unavailable, "Server Unavailable")
	}

	// start timing the service this is used later for the total request time
	ts := time.Now()
	finished := f.loadGenerator.Generate()
	defer finished()

	hq := f.log.HandleGRCPRequest(ctx)
	defer hq.Finished()

	resp := &response.Response{}
	resp.Name = f.name
	resp.Type = "gRPC"
	resp.IPAddresses = getIPInfo()

	// are we injecting errors, if so return the error
	if er := f.errorInjector.Do(); er != nil {
		resp.Code = er.Code
		resp.Error = er.Error.Error()

		hq.SetError(er.Error)
		hq.SetMetadata("response", strconv.Itoa(er.Code))

		// encode the response into the gRPC error message
		s := status.New(codes.Code(resp.Code), er.Error.Error())
		s, _ = s.WithDetails(&api.Response{Message: resp.ToJSON()})

		// return the error
		return nil, s.Err()
	}

	// if we need to create upstream requests create a worker pool
	var upstreamError error
	if len(f.upstreamURIs) > 0 {
		data := f.requestGenerator.Generate()
		wp := worker.New(f.workerCount, func(uri string) (*response.Response, error) {
			if strings.HasPrefix(uri, "http://") {
				return workerHTTP(hq.Span.Context(), uri, f.defaultClient, nil, f.log, data)
			}

			return workerGRPC(hq.Span.Context(), uri, f.grpcClients, f.log, data)
		})

		err := wp.Do(f.upstreamURIs)

		if err != nil {
			upstreamError = err
		}

		for _, v := range wp.Responses() {
			resp.AppendUpstream(v.URI, *v.Response)
		}
	}

	if upstreamError != nil {
		resp.Code = int(codes.Internal)
		resp.Error = upstreamError.Error()

		hq.SetMetadata("response", strconv.Itoa(resp.Code))
		hq.SetError(upstreamError)

		// encode the response into the gRPC error message
		s := status.New(codes.Code(resp.Code), upstreamError.Error())
		s, _ = s.WithDetails(&api.Response{Message: resp.ToJSON()})

		return nil, s.Err()
	}

	// service time is equal to the randomised time - the current time take
	d := f.duration.Calculate()
	et := time.Now().Sub(ts)
	rd := d - et
	if rd > 0 {
		// randomize the time the request takes
		lp := f.log.SleepService(hq.Span, rd)
		time.Sleep(rd)
		lp.Finished()
	}

	// log response code
	hq.SetMetadata("response", "0")

	// compute total elapsed time including duration
	te := time.Now()
	resp.StartTime = ts.Format(timeFormat)
	resp.EndTime = te.Format(timeFormat)
	resp.Duration = te.Sub(ts).String()

	// add the response body if there is no upstream error
	if upstreamError == nil {
		if strings.HasPrefix(f.message, "{") {
			resp.Body = json.RawMessage(f.message)
		} else {
			resp.Body = json.RawMessage(fmt.Sprintf(`"%s"`, f.message))
		}
	}

	return &api.Response{Message: resp.ToJSON()}, nil
}
