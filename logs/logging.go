package logs

import "encoding/json"

import "time"

// SysLogger writes to the system log.
type SysLogger interface {
	Error(v ...interface{}) error
	Warning(v ...interface{}) error
	Info(v ...interface{}) error

	Errorf(format string, a ...interface{}) error
	Warningf(format string, a ...interface{}) error
	Infof(format string, a ...interface{}) error
}

const (
	INFO    = "INFO"
	WARNING = "WARN"
	ERROR   = "ERR"
	DEBUG   = "DEBUG"
)

var (
	// Logger is anything that can handle the sysLogger interface
	debugLogging            = false
	jSONLogDefaults         = map[string]interface{}{}
	jSONPretty              = false
	jSONDebugLoggingEnabled = false
	DefaultLogger           SysLogger
)

type debugLogger struct {
	logger SysLogger
	debug  bool
}

var debuglogger debugLogger

// TurnDebuggingOn will tell the logger to log debug messages.
// They appear as info messages due to limits in the logging engine
// used to run the service.
func TurnDebuggingOn(logger SysLogger, debugging bool) {
	debuglogger = debugLogger{
		logger: logger,
		debug:  debugging,
	}
}

// DebugMessage send a debug message to the systems logger.
func DebugMessage(msg string) {
	if debuglogger.debug {
		debuglogger.logger.Info("[DEBUG]", msg)
	}
}

type JSONLogStructure struct {
	Message    string                 `json:"message"`
	Severity   string                 `json:"severity"`
	Time       string                 `json:"time"`
	Attributes map[string]interface{} `json:"attributes"`
}

type JSONAttributes map[string]interface{}

func SetJSONLogDefaults(defaults map[string]interface{}) {
	jSONLogDefaults = defaults
}

func OutputJSONPretty(b bool) {
	jSONPretty = b
}

func JSONDebugLogging(b bool) {
	jSONDebugLoggingEnabled = b
}

func copyOfJSONDefaults() map[string]interface{} {
	copy := map[string]interface{}{}
	for key, value := range jSONLogDefaults {
		copy[key] = value
	}
	return copy
}

func JSONLog(msg, severity string, attributes map[string]interface{}) {
	newLog := JSONLogStructure{
		Message:    msg,
		Severity:   severity,
		Attributes: copyOfJSONDefaults(),
		Time:       time.Now().Format("Mon Jan 2 15:04:05 -0700 MST 2006"),
	}
	for key, value := range attributes {
		newLog.Attributes[key] = value
	}

	var jsonBytes []byte
	var err error
	if jSONPretty {
		jsonBytes, err = json.MarshalIndent(newLog, "", "  ")
	} else {
		jsonBytes, err = json.Marshal(newLog)
	}
	if err != nil {
		errMessage := JSONLogStructure{
			Message:    "Failed to generate log",
			Severity:   ERROR,
			Attributes: map[string]interface{}{"incoming_message": msg},
		}
		severity = ERROR
		jsonBytes, _ = json.Marshal(errMessage)
	}

	switch severity {
	case INFO:
		DefaultLogger.Info(string(jsonBytes))
	case ERROR:
		DefaultLogger.Error(string(jsonBytes))
	case WARNING:
		DefaultLogger.Warning(string(jsonBytes))
	case DEBUG:
		if jSONDebugLoggingEnabled {
			DefaultLogger.Info(string(jsonBytes))
		}
	}

}
