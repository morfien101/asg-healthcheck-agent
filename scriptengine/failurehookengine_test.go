package scriptengine

import (
	"testing"

	"github.com/morfien101/asg-healthcheck-agent/config"
)

func TestFailureHookEngine(t *testing.T) {
	cfg := []config.FailureHook{
		config.FailureHook{
			Name:        "fail_1",
			Description: "Failure hook 1",
			Bin:         "/bin/bash",
			Args:        []string{"./testscript.sh", "0"},
			MaxRetry:    0,
		},
		config.FailureHook{
			Name:        "fail_2",
			Description: "Failure hook 2",
			Bin:         "/bin/bash",
			Args:        []string{"./testscript.sh", "1"},
			MaxRetry:    1,
		},
	}
	fhe := NewFailureHookEngine(cfg)

	fhe.RunHooks()
}
