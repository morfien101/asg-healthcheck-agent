package scriptengine

import (
	"time"

	"github.com/morfien101/asg-healthcheck-agent/config"
	"github.com/morfien101/asg-healthcheck-agent/logs"
	"github.com/morfien101/asg-healthcheck-agent/metrics"
)

type failureHookInterface interface {
	run()
}

type failureHook struct {
	Name                    string `json:"name"`
	Description             string `json:"description"`
	MaxRetry                uint   `json:"retries_allowed"`
	TimeBetweenRetrySeconds uint   `json:"seconds_between_retries"`
	bin                     string
	args                    []string
}

func newFailureHook(cfg config.FailureHook) *failureHook {
	return &failureHook{
		Name:                    cfg.Name,
		Description:             cfg.Description,
		MaxRetry:                cfg.MaxRetry,
		TimeBetweenRetrySeconds: cfg.WaitSecondsBetweenRetries,
		bin:                     cfg.Bin,
		args:                    cfg.Args,
	}
}

func metricFailureHookRanProcess(name string, exitcode int) {
	success := "true"
	if exitcode != 0 {
		success = "false"
	}
	metrics.Incr(
		"failure_hook_run",
		1,
		metrics.Tags{
			"successful": success,
			"name":       name,
		},
	)
}

func (fh *failureHook) run() {
	var attempt uint
	for attempt = 0; attempt <= fh.MaxRetry; attempt++ {
		if attempt != 0 {
			if fh.TimeBetweenRetrySeconds != 0 {
				time.Sleep(time.Duration(fh.TimeBetweenRetrySeconds) * time.Second)
			}
		}
		p, err := newProcess(fh.Name, fh.bin, fh.args...)
		if err != nil {
			logs.JSONLog(
				"Failed to create failure hook process",
				logs.ERROR,
				logs.JSONAttributes{
					"error":             err.Error(),
					"failure_hook_name": fh.Name,
				},
			)
			continue
		}
		exitcode, err := p.run()
		if err != nil {
			logs.JSONLog(
				"Failed to run failure hook process",
				logs.ERROR,
				logs.JSONAttributes{
					"error":             err.Error(),
					"failure_hook_name": fh.Name,
				},
			)
			continue
		}

		tryAgain := false
		if exitcode != 0 {
			logs.JSONLog(
				"Failure hook process exited bad",
				logs.WARNING,
				logs.JSONAttributes{
					"exitcode":          exitcode,
					"failure_hook_name": fh.Name,
				},
			)
			tryAgain = true
		}
		metricFailureHookRanProcess(fh.Name, exitcode)
		if tryAgain {
			continue
		}

		logs.JSONLog(
			"failure hook ran successfully",
			logs.INFO,
			logs.JSONAttributes{
				"failure_hook_name": fh.Name,
			},
		)
		break
	}
}
