package config

import "io/ioutil"

import "encoding/json"

import "os"

type Config struct {
	HealthChecks []HealthCheck `json:"health_checks"`
	FailureHooks []FailureHook `json:"failure_hooks"`
	// Wait x seconds before processing failures
	StartupGraceSeconds uint `json:"startup_grace_seconds"`
	// Should the failure hooks be run if the application stops because of a signal
	// Technically it didn't fail it was requested to stop.
	// Default is false
	RunFailureHooksOnTermSignal bool `json:"run_failure_hooks_on_term_signal"`
	// Should the failure hooks be run when detecting a failure.
	// Default is true
	RunFailureHooks bool `json:"run_failure_hooks"`
	// ExitAfterFailureHooks will cause the service to stop after the hooks
	// have finished running. Depending on the service manager, this
	// maybe undesirable as the service will just restart. If false
	// the service will sit idle till it is stopped.
	// Default is false.
	ExitAfterFailureHooks bool `json:"exit_after_failure_hooks"`
	// Pretty logs is used to make the logging engine pretty JSON logs.
	// Should only be used in testing situations.
	PrettyLogs bool `json:"pretty_logs"`
	DebugLogs  bool `json:"debug_logging"`
	// logging attributes will be added to all logs that are written after
	// successfully starting. These can be used to help identify logs for
	// groups of servers.
	DefaultLoggingAttributes map[string]string `json:"logging_attributes"`
	WebServer                WebServerConfig   `json:"webserver"`
	StatsD                   StatsDConfig      `json:"statsd"`
}

type WebServerConfig struct {
	Enabled     bool   `json:"enabled"`
	Address     string `json:"address"`
	Port        uint   `json:"port"`
	UseTLS      bool   `json:"use_tls"`
	TLSCertPath string `json:"cert_path"`
	TLSKeyPath  string `json:"key_path"`
	// returns nicely formatted JSON structures to the requester. This is useful for
	// reading as a human. Might be removed later for a flag in the query string.
	PrettyJSON bool `json:"pretty_json_responses"`
}

type StatsDConfig struct {
	Enabled bool   `json:"enabled"`
	Address string `json:"address"`
	Port    uint   `json:"port"`
	// If needed you can change the prefix that is used for statsd
	Prefix string `json:"prefix"`
	// default tags are tags that the user can specify in config and will be sent with all statsd messages
	DefaultTags map[string]string `json:"default_tags"`
}

// HealthCheck is a test to see if the server if functioning correctly.
type HealthCheck struct {
	// Used to show the check in the web server
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Bin         string   `json:"command"`
	Args        []string `json:"arguments"`
	// How often to run the checks.
	FreqSeconds uint `json:"frequency_in_seconds"`
	// How many failures can we accept before a stable failure is accepted
	AllowedFailures uint `json:"allowed_failures"`
	// If a check is failing, how any successes are required before the failure
	// count is reset to 0
	RecoverySuccessCount uint `json:"recovery_success_count"`
}

// FailureHook are scripts run when the health is changed to SICK.
// These can be used to change de-register instances from services or
// to change the termination life cycle hooks to proceed.
type FailureHook struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Bin         string   `json:"command"`
	Args        []string `json:"arguments"`
	// If set to more than 0, failures will be retied until the max is hit.
	MaxRetry                  uint `json:"max_retry"`
	WaitSecondsBetweenRetries uint `json:"seconds_between_retries"`
}

func newConfig() Config {
	defaultLoggingAttr := map[string]string{}
	defaultStatsdAttr := map[string]string{}

	if hn, err := os.Hostname(); err == nil {
		defaultLoggingAttr["hostname"] = hn
		defaultStatsdAttr["source"] = hn
	}

	cfg := Config{
		RunFailureHooksOnTermSignal: false,
		RunFailureHooks:             true,
		DefaultLoggingAttributes:    defaultLoggingAttr,
		HealthChecks:                []HealthCheck{},
		FailureHooks:                []FailureHook{},
		WebServer: WebServerConfig{
			Enabled:    true,
			Address:    "0.0.0.0",
			Port:       8011,
			UseTLS:     false,
			PrettyJSON: true,
		},
		StatsD: StatsDConfig{
			Enabled:     false,
			Address:     "127.0.0.1",
			Port:        8125,
			Prefix:      "asg_healthcheck",
			DefaultTags: defaultStatsdAttr,
		},
	}

	return cfg
}

// New creates a new Config and passes it back to the caller.
// Errors are also passed back and could include not being able to read the file from the disk
// or it is invalid.
func New(path string) (Config, error) {
	cfg := newConfig()
	cfgBytes, err := readConfigFile(path)
	if err != nil {
		return Config{}, err
	}
	err = mergeConfigs(cfgBytes, &cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func readConfigFile(path string) ([]byte, error) {
	return ioutil.ReadFile(path)
}

func mergeConfigs(configFileData []byte, defaultCfg *Config) error {
	return json.Unmarshal(configFileData, defaultCfg)
}

// This can be filled in later once we have more understanding of what we need.
// 24/02/2020
func validateConfig() error {
	return nil
}
