package scriptengine

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"syscall"

	"github.com/morfien101/asg-healthcheck-agent/logs"
)

const (
	stdoutString = "stdout"
	stderrString = "stderr"
)

type ProcessInterface interface {
	run()
}

type Process struct {
	name        string
	proc        *exec.Cmd
	stopLogging chan bool
}

// Setup Process will link create the process object and also link the stdout and stderr.
// An error is returned if anything fails.
func newProcess(name, bin string, args ...string) (*Process, error) {
	proc := &Process{
		name: name,
		proc: exec.Command(bin, args...),
	}

	procStdOut, err := proc.proc.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to stdout pipe. Error: %s", err)
	}

	procStdErr, err := proc.proc.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to stderr pipe. Error: %s", err)
	}
	go proc.pumpLogs(procStdOut, procStdErr)

	return proc, nil
}

func (proc *Process) pumpLogs(stdout, stderr io.ReadCloser) {
	sendLog := func(message string, pipe string) {
		sev := logs.WARNING
		if pipe == stderrString {
			sev = logs.ERROR
		}
		logs.JSONLog(
			message,
			sev,
			logs.JSONAttributes{
				"pipe":         pipe,
				"process_name": proc.name,
			},
		)
	}

	proc.stopLogging = make(chan bool, 1)
	go func() {
		<-proc.stopLogging
		defer stdout.Close()
		defer stderr.Close()
	}()

	stdOutScanner := bufio.NewScanner(stdout)
	stdErrScanner := bufio.NewScanner(stderr)

	go func() {
		for stdOutScanner.Scan() {
			sendLog(stdOutScanner.Text(), stdoutString)
		}
	}()
	go func() {
		for stdErrScanner.Scan() {
			sendLog(stdErrScanner.Text(), stderrString)
		}
	}()
}

func (proc *Process) run() (exitcode int, err error) {
	// Everything is bad until the process exits successfully.
	exitcode = 1
	if err := proc.proc.Start(); err != nil {
		return 1, err
	}
	// Wait for the process to finish
	procComplete := make(chan error, 1)
	go func() {
		procComplete <- proc.proc.Wait()
		// Close the pipes that redirect std out and err
		proc.stopLogging <- true
	}()

	exitError := <-procComplete
	if exiterr, ok := exitError.(*exec.ExitError); ok {
		// The program has exited with an exit code != 0

		// This works on both Unix and Windows. Although package
		// syscall is generally platform dependent, WaitStatus is
		// defined for both Unix and Windows and in both cases has
		// an ExitStatus() method with the same signature.
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			exitstatus := status.ExitStatus()
			if exitstatus == -1 {
				exitstatus = 1
			}
			exitcode = exitstatus
		} else {
			exitcode = 1
		}
	} else {
		exitcode = 0
	}

	return exitcode, nil
}
