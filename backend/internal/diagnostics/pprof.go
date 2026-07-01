// Package diagnostics owns optional runtime diagnostic listeners.
package diagnostics

import (
	"net"
	"net/http"
	nhpprof "net/http/pprof"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	glog "github.com/Laisky/go-utils/v6/log"
	"github.com/Laisky/zap"
)

// StartPprofServer receives a listen address and logger, starts a dedicated pprof listener, and returns the server.
func StartPprofServer(addr string, log glog.Logger) (*http.Server, error) {
	if log == nil {
		return nil, errors.New("logger is nil")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", nhpprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", nhpprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", nhpprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", nhpprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", nhpprof.Trace)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if !IsLoopbackListenAddr(addr) {
		log.Warn("pprof listener is bound to a non-loopback address; pprof has no authentication, ensure it is protected",
			zap.String("address", addr))
	}

	go func() {
		log.Info("pprof server started", zap.String("address", "http://"+addr+"/debug/pprof/"))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("pprof server stopped unexpectedly", zap.Error(err))
		}
	}()

	return server, nil
}

// IsLoopbackListenAddr receives a listen address and returns whether it binds only to loopback.
func IsLoopbackListenAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = strings.TrimSpace(addr)
	}

	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}

	return false
}
