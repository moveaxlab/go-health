package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Checker func(context.Context) CheckStatus

type CheckProvider interface {
	HealthCheck() Checker
}

type MultiCheckProvider interface {
	HealthChecks() []Checker
}

type CheckServer interface {
	Start()
	Check(checkProvider CheckProvider)
	CheckAll(checkProvider MultiCheckProvider)
	AddHealthChecker(checker ...Checker)
	Stop() error
}

type healthCheckResponse struct {
	Status  string            `json:"status"`
	Info    map[string]string `json:"info,omitempty"`
	Error   string            `json:"error,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

type CheckStatus struct {
	Name    string
	Healthy bool
	Info    string
	Details string
	Error   string
}

type checkServer struct {
	server         *http.Server
	wg             *sync.WaitGroup
	healthCheckers []Checker
}

func NewHealthCheckServer(port int) CheckServer {
	res := &checkServer{
		server:         &http.Server{Addr: fmt.Sprintf(":%d", port)},
		wg:             &sync.WaitGroup{},
		healthCheckers: make([]Checker, 0),
	}

	http.HandleFunc("/ready", res.readiness)
	http.HandleFunc("/live", res.liveness)

	return res
}

func (s *checkServer) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		err := s.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("health server failed: %v", err)
		}
	}()

	log.Infof("health server listening on port %s", s.server.Addr)
}

func (s *checkServer) Check(checkProvider CheckProvider) {
	s.AddHealthChecker(checkProvider.HealthCheck())
}

func (s *checkServer) CheckAll(checkProvider MultiCheckProvider) {
	s.AddHealthChecker(checkProvider.HealthChecks()...)
}

func (s *checkServer) Stop() error {
	log.Debugf("stopping health server...")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *checkServer) AddHealthChecker(checker ...Checker) {
	s.healthCheckers = append(s.healthCheckers, checker...)
}

func (s *checkServer) liveness(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(200)
	_, _ = fmt.Fprint(w, "live")
}

func (s *checkServer) readiness(w http.ResponseWriter, req *http.Request) {
	ch := make(chan CheckStatus, len(s.healthCheckers))
	defer close(ch)

	for _, f := range s.healthCheckers {
		go func(f Checker) {
			res := f(req.Context())
			ch <- res
		}(f)
	}

	result := healthCheckResponse{
		Status:  "up",
		Info:    make(map[string]string),
		Error:   "",
		Details: make(map[string]string),
	}

	for range s.healthCheckers {
		response := <-ch
		if !response.Healthy {
			result.Error = response.Error
			result.Status = "down"
		} else {
			result.Info[response.Name] = response.Info
			result.Details[response.Name] = response.Details
		}
	}

	w.Header().Set("Content-Type", "application/json")

	res, err := json.Marshal(result)
	if err != nil {
		w.WriteHeader(503)
		_, _ = fmt.Fprintf(w, "unhealthy: %v", err)
		return
	}

	if result.Status == "up" {
		w.WriteHeader(200)
	} else {
		w.WriteHeader(503)
	}

	_, _ = w.Write(res)
}
