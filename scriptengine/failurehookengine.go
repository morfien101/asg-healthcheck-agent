package scriptengine

import "github.com/morfien101/asg-healthcheck-agent/config"

// FailureHookEngineInterface describes how a FailureHookEngine will work
type FailureHookEngineInterface interface {
	RunHooks()
}

// FailureHookEngine will run the failure hooks when required.
type FailureHookEngine struct {
	FailureHooks []*failureHook
}

// NewFailureHookEngine will populate a new failure hook engine
// and return a pointer to it.
func NewFailureHookEngine(cfg []config.FailureHook) *FailureHookEngine {
	fhe := &FailureHookEngine{
		FailureHooks: []*failureHook{},
	}
	for _, fhConfig := range cfg {
		fhe.FailureHooks = append(fhe.FailureHooks, newFailureHook(fhConfig))
	}

	return fhe
}

// RunHooks will run each of the hooks in sequence.
// Hooks will retry if failed and will not return errors.
// The failure hooks are expected to be run as the last action in the chain
// so dealing with errors besides logging is pointless.
func (fhe *FailureHookEngine) RunHooks() {
	for _, hook := range fhe.FailureHooks {
		hook.run()
	}
}
