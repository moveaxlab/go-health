package health

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const healthPort = 3000

var healthAddress = fmt.Sprintf("0.0.0.0:%d", healthPort)

var server CheckServer

func init() {
	server = NewHealthCheckServer(healthPort)
}

func waitForServer() {
	for {
		_, err := http.Get(fmt.Sprintf("http://%s/live", healthAddress))
		if err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type mockHealthChecker struct {
	mock.Mock
}

func (m *mockHealthChecker) Check(ctx context.Context) CheckStatus {
	return m.Called(ctx).Get(0).(CheckStatus)
}

func TestHealthServer(t *testing.T) {
	healthChecker := &mockHealthChecker{}

	healthChecker.
		On("Check", mock.Anything).
		Return(CheckStatus{
			Name:    "test",
			Healthy: true,
		}).
		Once()

	server.AddHealthChecker(healthChecker.Check)

	server.Start()

	waitForServer()

	resp, err := http.Get(fmt.Sprintf("http://%s/ready", healthAddress))

	assert.Nil(t, err)

	assert.Equal(t, 200, resp.StatusCode)

	healthChecker.
		On("Check", mock.Anything).
		Return(CheckStatus{
			Name:    "test",
			Healthy: false,
		}).
		Once()

	resp, err = http.Get(fmt.Sprintf("http://%s/ready", healthAddress))

	assert.Nil(t, err)

	assert.Equal(t, 503, resp.StatusCode)

	assert.Nil(t, server.Stop())
}
