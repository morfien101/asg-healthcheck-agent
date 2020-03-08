package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/morfien101/asg-healthcheck-agent/config"
	"github.com/morfien101/asg-healthcheck-agent/logs"
	"github.com/morfien101/asg-healthcheck-agent/statemanager"
)

type HTTPEngine struct {
	router       *mux.Router
	server       *http.Server
	stateManager *statemanager.StateManager
	running      bool
}

func New(cfg config.WebServerConfig, sm *statemanager.StateManager) *HTTPEngine {
	httpEngine := &HTTPEngine{
		stateManager: sm,
		router:       mux.NewRouter(),
	}
	httpEngine.router.HandleFunc("/_status", httpEngine.showStatus).Methods("Get")

	return httpEngine
}

// StartHTTPEngine will start the web server in a nonTLS mode.
// It also requires that the listening address be passes in as a string.
// Should be used in a go routine.
func (e *HTTPEngine) StartHTTPEngine(listenerAddress string) error {
	// Start the HTTP Engine
	e.server = &http.Server{Addr: listenerAddress, Handler: e.router}
	return e.server.ListenAndServe()
}

// StartHTTPSEngine will start the web server with TLS support using the given cert and key values.
// It also requires that the listening address be passes in as a string.
// Should be used in a go routine.
func (e *HTTPEngine) StartHTTPSEngine(listenerAddress, certPath, keyPath string) error {
	// Start the HTTP Engine
	e.server = &http.Server{Addr: listenerAddress, Handler: e.router}
	return e.server.ListenAndServeTLS(certPath, keyPath)
}

// StopHTTPEngine will stop the web server grafefully.
// It will give the server 5 seconds before just terminating it.
func (e *HTTPEngine) StopHTTPEngine() error {
	// Stop the HTTP Engine
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()
	return e.server.Shutdown(ctx)
}

// ServeHTTP is used to allow the router to start accepting requests before the start is started up. This will help with testing.
func (e *HTTPEngine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.router.ServeHTTP(w, r)
}

func setContentJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
}

func jsonMarshal(x interface{}) ([]byte, error) {
	return json.MarshalIndent(x, "", "  ")
}

func printJSON(w http.ResponseWriter, jsonbytes []byte) (int, error) {
	return fmt.Fprint(w, string(jsonbytes), "\n")
}

// showStatus will show the status of the sever
func (e *HTTPEngine) showStatus(w http.ResponseWriter, r *http.Request) {
	if !e.stateManager.Healthy {
		w.WriteHeader(http.StatusInternalServerError)
	}

	respBytes, err := json.Marshal(e.stateManager)
	if err != nil {
		respBytes = []byte("Internal server error")
		w.WriteHeader(http.StatusInternalServerError)
		logs.JSONLog(
			"Error decoding the state manager to JSON",
			logs.ERROR,
			logs.JSONAttributes{"error": err.Error()},
		)
	}
	setContentJSON(w)
	fmt.Fprint(w, string(respBytes))
}
