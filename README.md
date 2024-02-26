# Go health server

This repo contains a liveness and readiness server for Go.

## Installation

```bash
go get github.com/moveaxlab/go-health
```

## Usage

Health checks are defined with functions that take in input a `context.Context`
and return a `CheckStatus` struct.
The struct contains a `Healthy` boolean to indicate wether the monitored feature
is healthy, and can provide additional info in the `Info` and `Details` fields.
The `Error` field can contain additional info when the feature is unhealthy.

Health checks can be registered on the health check server,
and will be called every time the `/ready` endpoint is called.

The `/ready` endpoint will return a `200` HTTP status if all health checks
report a healthy status, or a `503` HTTP status otherwise.
The response body will contain details on the health status of the service.

The `/live` endpoint always returns a `200` HTTP status.

Your services can implement the `HealthCheck` and `HealthChecks` methods
that return respectively a single checker function or multiple checker functions.
Services that implement this interface can be registered with the health check server
using the `Check` and `CheckAll` methods.

```go
package main

import (
	"os"
	"sync"
	"syscall"

	"github.com/moveaxlab/go-health"
)

type MyController interface {
	HealthCheck() health.Checker
}
	
type MyDatabase interface {
	HealthChecks() []health.Checker
}

func main() {
	server := health.NewHealthCheckServer(8080)
	wg := &sync.WaitGroup{}
	channel := make(chan os.Signal, 1)

	// register a custom health checker
	server.AddHealthChecker(func (_ context.Context) health.CheckStatus {
		return {
			Name: "custom",
			Healthy: true,
		}
	})

	var controller MyController
	var database MyDatabase

	// or register your services
	server.Check(controller)
	server.CheckAll(database)

	// start the server
	server.Start()

	// stop the server gracefully
	signal.Notify(channel, syscall.SIGINT)
	signal.Notify(channel, syscall.SIGTERM)

	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-channel:
			err := server.Stop()
			if err != nil {
				fmt.Printf("server stop returned an error: %v", err)
			}
		}

	}()
	wg.Wait()
}
```
