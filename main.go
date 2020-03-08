package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/morfien101/asg-healthcheck-agent/config"
	"github.com/morfien101/asg-healthcheck-agent/logs"
	"github.com/morfien101/asg-healthcheck-agent/metrics"
	"github.com/morfien101/asg-healthcheck-agent/scriptengine"
	"github.com/morfien101/asg-healthcheck-agent/statemanager"
	"github.com/morfien101/asg-healthcheck-agent/webserver"
	"github.com/morfien101/service"
)

// VERSION holds the version of the program
// Don't change this as the build server tags the builds.
var VERSION = "0.0.2"
var defaultConfigLocation = "/etc/asg-healthchecker/config.json"

// Flags for the application launch
var (
	versionCheck        = flag.Bool("v", false, "Outputs the version of the program.")
	helpFlag            = flag.Bool("h", false, "Shows the help menu.")
	configLocaltionFlag = flag.String("c", defaultConfigLocation, "Location of the configuration file.")
	showconfigFlag      = flag.Bool("s", false, "Show full running config")
	svcFlag             = flag.String("service", "", "Control the system service.")
)

type configStop error

type program struct {
	exit           chan bool
	finshed        chan bool
	configLocation string
	config         config.Config
	signalsChan    chan os.Signal
}

func main() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	// Deal with flags
	digestFlags()

	svcConfig := &service.Config{
		Name:        "asg_healthchecker",
		DisplayName: "ASG Health Checker",
		Description: "Health check runner for auto scaling group instances",
	}

	prg := &program{
		configLocation: *configLocaltionFlag,
		signalsChan:    signals,
	}

	serviceController, err := service.New(prg, svcConfig)
	if err != nil {
		// TODO: should print JSON log
		log.Fatal(err)
	}

	// Create a channel to house the errors from the service.
	// These are then printed out on the go func below.
	errsChan := make(chan error, 5)
	logger, err := serviceController.Logger(errsChan)
	if err != nil {
		// TODO: Should print json
		log.Fatal(err)
	}
	logs.DefaultLogger = logger

	go func() {
		for {
			err := <-errsChan
			if err != nil {
				logs.JSONLog(
					err.Error(),
					logs.ERROR,
					logs.JSONAttributes{},
				)
			}
		}
	}()

	// Look for the --service flag and do what we need to here.
	if len(*svcFlag) != 0 {
		err := service.Control(serviceController, *svcFlag)
		if err != nil {
			fmt.Printf("Valid actions: %q\n", service.ControlAction)
			log.Fatal(err)
		}
		return
	}
	err = serviceController.Run()
	if err != nil {
		logger.Error(err)
	}
}

func digestFlags() {
	// Parse the flags to get the state we need to run in.
	flag.Parse()

	if *versionCheck {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	if *helpFlag {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *showconfigFlag {
		cfg, err := generateConfig(*configLocaltionFlag)
		if err != nil {
			log.Fatalf("Failed to generate config. Error: %s", err)
			os.Exit(0)
		}
		b, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			log.Fatalf("Failed to generate config. Error: %s", err)
		}
		fmt.Println(string(b))
		os.Exit(0)
	}
}

func generateConfig(path string) (config.Config, error) {
	return config.New(path)
}

func (p *program) Stop(s service.Service) error {
	// This section is to shutdown the app gracefully.
	// return any errors relating to the above.
	// Send your shutdown signals to any other go routines or work flows from here.

	// This channel is used in the running section to block.
	p.exit <- true
	close(p.exit)

	return nil
}

func (p *program) Start(s service.Service) error {
	// This channel is used in the run section to block.
	// exit is used to determine if the exit is caused by a signal or an error.
	// If a signal is used then the value is expected to be true.
	p.exit = make(chan bool, 1)
	p.finshed = make(chan bool, 1)
	go func() {
		<-p.signalsChan
		p.exit <- true
	}()

	// Errors would relate to looking for config files.
	config, err := generateConfig(p.configLocation)
	if err != nil {
		return err
	}
	p.config = config

	// Configure the logger since we now know what it should look like.
	logs.JSONDebugLogging(config.DebugLogs)
	logs.OutputJSONPretty(config.PrettyLogs)
	jsonDefaults := map[string]interface{}{}
	for key, value := range config.DefaultLoggingAttributes {
		jsonDefaults[key] = value
	}
	logs.SetJSONLogDefaults(jsonDefaults)

	if config.StatsD.Enabled {
		metrics.Setup(
			fmt.Sprintf("%s:%d", config.StatsD.Address, config.StatsD.Port),
			config.StatsD.Prefix,
			config.StatsD.DefaultTags,
		)
		metrics.Enable()
	}
	metrics.Incr("starting", 1, metrics.Tags{})
	// Start the service in a async go routine
	go p.run()
	go func() {
		exitcode := 0
		ok := <-p.finshed
		if !ok {
			exitcode = 1
		}
		os.Exit(exitcode)
	}()

	// return any errors.
	return nil
}

func (p *program) run() error {

	hce := scriptengine.NewHealthCheckEngine(p.config.HealthChecks)
	fhe := scriptengine.NewFailureHookEngine(p.config.FailureHooks)
	statemanager := statemanager.New(hce, fhe, p.config.RunFailureHooksOnTermSignal, p.config.RunFailureHooks)
	websrv := webserver.New(p.config.WebServer, &statemanager)
	fatalErrors := make(chan error, 1)

	go func() {
		fatalErrors <- websrv.StartHTTPEngine("127.0.0.1:8080")
	}()
	go func() {
		stateManagerErrorChan := statemanager.Start(p.config.StartupGraceSeconds)
		select {
		case err, ok := <-stateManagerErrorChan:
			if !ok {
				// No error arrived, do we need to stop
				if p.config.ExitAfterFailureHooks {
					fatalErrors <- configStop(fmt.Errorf("stopping due to configuration instruction"))
				}
			}
			fatalErrors <- err
		}
	}()

	for {
		select {
		case err := <-fatalErrors:
			if err != nil {
				if _, ok := err.(configStop); ok {
					p.exit <- false
				} else {
					logs.JSONLog(
						"Fatal Error detected",
						logs.ERROR,
						logs.JSONAttributes{"error": err.Error()},
					)
					p.exit <- false
				}
			}
		case isSignal := <-p.exit:
			// Shutdown everything here
			if isSignal {
				logs.JSONLog(
					"Stop signal caught, attempting to stop.",
					logs.INFO,
					logs.JSONAttributes{},
				)
			}
			statemanager.Stop(isSignal)
			if err := websrv.StopHTTPEngine(); err != nil {
				logs.JSONLog(
					"Failed to stop web server",
					logs.ERROR,
					logs.JSONAttributes{"error": err.Error()},
				)
			}
			metrics.Incr("stopping", 1, metrics.Tags{})
			p.finshed <- true
			return nil
		}
	}
}
