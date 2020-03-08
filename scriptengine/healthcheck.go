package scriptengine

import (
	"sync"
	"time"

	"github.com/morfien101/asg-healthcheck-agent/config"
	"github.com/morfien101/asg-healthcheck-agent/logs"
	"github.com/morfien101/asg-healthcheck-agent/metrics"
)

type HealthCheckInternface interface {
	Start()
	Stop()
}

// HealthCheck is a single health check.
// It is used to run the checks on the servers.
type HealthCheck struct {
	Name                     string `json:"name"`
	Description              string `json:"description"`
	LastExitCode             int    `json:"last_exit_code"`
	LastRuntime              string `json:"last_run_time"`
	TotalFailureCount        uint   `json:"failure_count"`
	RecoveryAttempt          uint   `json:"recovery_attempt"`
	FailureSinceLastRecovery uint   `json:"failures_since_last_recovery"`
	FreqSeconds              uint   `json:"frequency_seconds"`
	AllowedFailures          uint   `json:"allowed_failures"`
	RecoveriesRequired       uint   `json:"recovery_count_required"`
	GraceMode                bool   `json:"grace_mode"`
	failureCounter           uint
	stdErr                   chan string
	stdout                   chan string
	bin                      string
	args                     []string
	runChecks                chan struct{}
	failedChan               chan<- string
	running                  bool
	lock                     sync.RWMutex
}

func newHealthCheck(publishFailuresOn chan<- string, cfg config.HealthCheck) *HealthCheck {
	return &HealthCheck{
		Name:               cfg.Name,
		Description:        cfg.Description,
		GraceMode:          true,
		LastExitCode:       -1,
		LastRuntime:        "never",
		FreqSeconds:        cfg.FreqSeconds,
		AllowedFailures:    cfg.AllowedFailures,
		RecoveriesRequired: cfg.RecoverySuccessCount,
		bin:                cfg.Bin,
		args:               cfg.Args,
		stdErr:             make(chan string, 10),
		stdout:             make(chan string, 10),
		runChecks:          make(chan struct{}, 1),
		failedChan:         publishFailuresOn,
	}
}

func metricHealthcheckRanProcess(name string, exitcode int) {
	success := "true"
	if exitcode != 0 {
		success = "false"
	}
	metrics.Incr(
		"healthcheck_run",
		1,
		metrics.Tags{
			"successful": success,
			"name":       name,
		},
	)
}

// Start will instruct the health check to run on the schedules given
func (hc *HealthCheck) Start() {
	ticker := time.NewTicker(time.Duration(hc.FreqSeconds) * time.Second)
	hc.running = true
	go func() {
		for {
			select {
			// When hc.runChecks is closed, consider the checks as not being required
			// anymore. Therefore we can stop the ticker and return the go func.
			case _, ok := <-hc.runChecks:
				if !ok {
					ticker.Stop()
					return
				}
			case <-ticker.C:
				// Do a check here.
				logs.JSONLog(
					"Attempting to run healthcheck",
					logs.DEBUG,
					logs.JSONAttributes{
						"healthcheck_name": hc.Name,
					},
				)

				check, err := newProcess(hc.Name, hc.bin, hc.args...)
				if err != nil {
					logs.JSONLog(
						"failed to create process",
						logs.ERROR,
						logs.JSONAttributes{
							"healthcheck_name": hc.Name,
							"error":            err.Error(),
						},
					)
					hc.LastExitCode = 1
					continue
				}
				exitcode, err := check.run()
				if err != nil {
					logs.JSONLog(
						"failed to run process",
						logs.ERROR,
						logs.JSONAttributes{
							"healthcheck_name": hc.Name,
							"error":            err.Error(),
						},
					)
					hc.LastExitCode = 1
					return
				}
				hc.LastExitCode = exitcode
				hc.LastRuntime = time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006")
				if !hc.GraceMode {
					hc.determineFailure()
					metricHealthcheckRanProcess(hc.Name, hc.LastExitCode)
				}
			}
		}
	}()
}

func (hc *HealthCheck) determineFailure() {
	// Was the last check a failure
	if hc.LastExitCode != 0 {
		hc.TotalFailureCount++
		hc.FailureSinceLastRecovery++

		// Is the failure count now higher than allowed failures?
		if hc.FailureSinceLastRecovery > hc.AllowedFailures {
			// If so consider this a stable failure
			hc.failedChan <- hc.Name
		}
		// A failure will reset the recovery back to 0 to stop flapping checks.
		// Checks should be stable and if they flap its just as bad as failures.
		if hc.RecoveryAttempt != 0 {
			hc.RecoveryAttempt = 0
		}

		return
	}

	// If the failure count is higher than zero, we need to see if we need to reset it as a
	// recovered service.
	if hc.FailureSinceLastRecovery > 0 {
		hc.RecoveryAttempt++
		if hc.RecoveryAttempt >= hc.RecoveriesRequired {
			// We have a stable recovery.
			// reset counters and move on.
			hc.FailureSinceLastRecovery = 0
			hc.RecoveryAttempt = 0
		}
	}
}

// Stop will instruct the health check to stop running on the interval.
// If a check is in running the moment this is called, that check will continue until it is finished
// then further runs will be stopped.
func (hc *HealthCheck) Stop() {
	hc.lock.Lock()
	if hc.running {
		hc.running = false
		close(hc.runChecks)
	}
	hc.lock.Unlock()
}
