package scriptengine

import (
	"github.com/morfien101/asg-healthcheck-agent/config"
)

type HealthCheckEngineInterface interface {
	Start(chan<- string)
	SetGraceMode(bool)
	Stop()
}

// HealthCheckEngine is used to run the health checks.
// It will send a message on the externalFailureChannel when a stable failure is found.
type HealthCheckEngine struct {
	internalFailureChan chan string
	HealthChecks        []*HealthCheck `json:"health_checks"`
}

// NewHealthCheckEngine return a new a pointer struct will run health checks
func NewHealthCheckEngine(cfg []config.HealthCheck) *HealthCheckEngine {
	hce := &HealthCheckEngine{
		internalFailureChan: make(chan string, 1),
		HealthChecks:        []*HealthCheck{},
	}
	for _, healthCheckConfig := range cfg {
		hce.HealthChecks = append(
			hce.HealthChecks,
			newHealthCheck(hce.internalFailureChan, healthCheckConfig),
		)
	}

	return hce
}

// SetGraceMode will tell the engine to forward on stable failures to the state
// manager via the channel. This is used to allow the grace period when checks
// might fail because the servier is not ready to service traffic.
func (hce *HealthCheckEngine) SetGraceMode(action bool) {
	for _, hc := range hce.HealthChecks {
		hc.GraceMode = action
	}
}

// Start instructs the health check engine to start running checks and to collect the failures.
// Failures are only forwarded once ProcessFailure is set to true
func (hce *HealthCheckEngine) Start(ExternalFailureChannel chan<- string) {
	// Plumb pipes for sending failures onwards.
	go func() {
		select {
		case FailureName, ok := <-hce.internalFailureChan:
			if !ok {
				return
			}
			ExternalFailureChannel <- FailureName
		}
	}()
	// Start the health checks
	for _, hc := range hce.HealthChecks {
		hc.Start()
	}
}

// Stop will instruct the health check engine to stop all health checks.
// It will also set the processFailure to false, to stop any future failures from
// coming in.
func (hce *HealthCheckEngine) Stop() {
	for _, hc := range hce.HealthChecks {
		hc.Stop()
	}
}
