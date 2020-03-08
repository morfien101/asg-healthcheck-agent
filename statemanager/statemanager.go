package statemanager

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/morfien101/asg-healthcheck-agent/logs"
	"github.com/morfien101/asg-healthcheck-agent/metrics"
	"github.com/morfien101/asg-healthcheck-agent/scriptengine"
)

// State is stored in the overall health.
// Once the health checks fail an instance is set to be terminated.
// Script engines will test health until it gets a stable failure on ANY test.
// This stable failure will cause the health to change to SICK.
//
// Changing health to SICK will cause the failure hooks to start running.
// While failure hooks start the ASG manager will set the instance health in AWS ASG to unhealthy.
// It's expected that a Termination life cycle hook is in place and that the last hook will set
// the life cycle hook to proceed.
// If not then the ASG will just terminate the instance at will and the hooks will not have time
// in some cases to complete.

type StateManager struct {
	failureChan         chan string
	exitChan            chan error
	metricHeartBeatChan chan struct{}
	// We may not want to run the hooks if we get a signal to terminate.
	runFailureHooksOnSignal bool
	runFailureHooks         bool
	Healthy                 bool                                    `json:"healthy"`
	HealthCheckEngine       scriptengine.HealthCheckEngineInterface `json:"health_checks"`
	FailureHookEngine       scriptengine.FailureHookEngineInterface `json:"failure_hooks"`
}

// New returns a StateManager that has been populated the with the supplied values.
func New(
	hce scriptengine.HealthCheckEngineInterface,
	fhe scriptengine.FailureHookEngineInterface,
	runHooksOnSignal bool,
	runHooks bool,
) StateManager {
	sm := StateManager{
		Healthy:                 true,
		failureChan:             make(chan string, 1),
		exitChan:                make(chan error, 1),
		runFailureHooksOnSignal: runHooksOnSignal,
		runFailureHooks:         runHooks,
		HealthCheckEngine:       hce,
		FailureHookEngine:       fhe,
	}

	return sm
}

// Start will run the underlying processes to enable health monitoring.
func (sm *StateManager) Start(gracePeriod uint) <-chan error {
	// Start the underlying processes
	sm.HealthCheckEngine.Start(sm.failureChan)

	go func() {
		if gracePeriod > 0 {
			time.Sleep(time.Second * time.Duration(gracePeriod))
		}
		sm.HealthCheckEngine.SetGraceMode(false)
	}()

	go sm.readFromFailChan()
	sm.metricHeartBeatChan = sm.startMetricsHeartBeat()
	return sm.exitChan
}

// Stop will tell the statemanager to stop the healthcheck engine and depending on the config
// for termination signal handleing, it will also run the failure hooks.
// It maybe undesirable for failure hooks to be run on a termination signal as it is a
// controlled shutdown and not really a failure. However the option exists to run them if required.
// Beware that failure hooks could potentially take a long time to run and will affect the shutdown
// time for the program.
func (sm *StateManager) Stop(singalTermination bool) {
	sm.HealthCheckEngine.Stop()
	if singalTermination {
		if sm.runFailureHooksOnSignal {
			sm.FailureHookEngine.RunHooks()
		}
	}
	close(sm.metricHeartBeatChan)
}

func (sm *StateManager) readFromFailChan() {
	for {
		select {
		case failureName := <-sm.failureChan:
			sm.actionFailure(failureName)
		}
	}
}

func (sm *StateManager) actionFailure(failureCause string) {
	if !sm.Healthy {
		return
	}
	// Got a stable failure.
	// Set Healthy false
	sm.Healthy = false
	logs.JSONLog(
		"Stable failure detected",
		logs.WARNING,
		logs.JSONAttributes{
			"check_name": failureCause,
		},
	)
	sm.HealthCheckEngine.Stop()
	// Process failure hooks
	if sm.runFailureHooks {
		sm.FailureHookEngine.RunHooks()
	}
	// We are finished so we can close the exit chan to indicate this.
	close(sm.exitChan)
}

func (sm *StateManager) startMetricsHeartBeat() chan struct{} {
	rand.Seed(time.Now().Unix())
	ticker := time.NewTicker(time.Second * 5)
	stopChan := make(chan struct{}, 1)
	go func() {
		metricName := "heartbeat"
		tags := metrics.Tags{"healthy": fmt.Sprintf("%v", sm.Healthy)}
		metrics.Gauge(metricName, 0, tags)
		for {
			select {
			case _, ok := <-stopChan:
				if !ok {
					ticker.Stop()
					return
				}
			case <-ticker.C:
				metrics.Gauge(metricName, rand.Int63n(100), tags)
			}
		}
	}()
	return stopChan
}
