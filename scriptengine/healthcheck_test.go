package scriptengine

import (
	"testing"
	"time"
)

// Test to see if a failure with no recovery works
func TestSingleFailureOnHealthCheck(t *testing.T) {
	failchan := make(chan string, 1)
	hc := HealthCheck{
		LastExitCode:    1,
		AllowedFailures: 0,
		failedChan:      failchan,
	}

	hc.determineFailure()

	select {
	case b := <-failchan:
		t.Logf("Got a %s on failure channel", b)
	default:
		t.Fail()
		t.Log("Expected to get an error on the failure channel")
	}
}

func TestMultipleFailureOnHealthCheck(t *testing.T) {
	failchan := make(chan string, 1)
	hc := HealthCheck{
		LastExitCode:    1,
		AllowedFailures: 1,
		failedChan:      failchan,
	}
	// First Failure
	hc.determineFailure()

	select {
	case b := <-failchan:
		t.Logf("Got a %s on failure channel, expected nothing", b)
		t.Fail()
	default:
		t.Log("First failure, not expecting a message.")
	}

	// Second failure. The exitcode is still 1, so its still a failure.
	hc.determineFailure()

	select {
	case b := <-failchan:
		t.Logf("Got a %s on failure channel", b)
	default:
		t.Fail()
		t.Log("Expected to get an error on the failure channel at second failure")
	}
}

func TestRecoveryOnHealthCheck(t *testing.T) {
	failchan := make(chan string, 1)
	hc := HealthCheck{
		LastExitCode:       1,
		AllowedFailures:    2,
		RecoveriesRequired: 1,
		failedChan:         failchan,
	}
	hc.determineFailure()

	select {
	case b := <-failchan:
		t.Logf("Got a %s on failure channel, expected nothing", b)
		t.Fail()
	default:
		t.Log("First failure, not expecting a message.")
	}

	// Set a good run
	hc.LastExitCode = 0
	hc.determineFailure()

	select {
	case b := <-failchan:
		t.Logf("Got a %s on failure channel, recovery should not trigger a failure", b)
		t.Fail()
	default:
		t.Log("Recovery successful")
	}
	if hc.failureCounter > 0 {
		t.Log("Recovery should reset the failure counter")
		t.Fail()
	}
}

func TestBashOnHealthCheck(t *testing.T) {
	failchan := make(chan string, 1)
	hc := HealthCheck{
		Name:               "Test Bash",
		bin:                "/bin/bash",
		args:               []string{"./testscript.sh", "0"},
		FreqSeconds:        1,
		AllowedFailures:    1,
		RecoveriesRequired: 1,
		failedChan:         failchan,
		runChecks:          make(chan struct{}, 1),
	}

	hc.Start()
	time.Sleep(time.Second * 3)
	hc.args = []string{"./testscript.sh", "1"}
	dead := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case b, ok := <-failchan:
				if !ok {
					dead <- struct{}{}
					break
				}
				t.Logf("Got a %s on failure channel", b)
				hc.Stop()
				close(failchan)
			}
		}
	}()
	<-dead
}
