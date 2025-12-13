package main

import (
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"bearclave-examples/internal/setup"

	"github.com/tahardi/bearclave/tee"
)

var configFile string

func main() {
	flag.StringVar(
		&configFile,
		"config",
		"configs/enclave/notee.yaml",
		"The Trusted Computing platform to use. Options: "+
			"nitro, sev, tdx, notee (default: notee)",
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config, err := setup.LoadConfig(configFile)
	if err != nil {
		logger.Error("loading config", slog.String("error", err.Error()))
		return
	}
	logger.Info("loaded config", slog.Any(configFile, config))

	proxy, err := tee.NewReverseProxy(config.Platform, config.Enclave.Addr, config.Proxy.Route)
	if err != nil {
		logger.Error("making reverse proxy", slog.String("error", err.Error()))
		return
	}

	server := &http.Server{
		Addr:    "0.0.0.0:8080",
		Handler: proxy,
		MaxHeaderBytes: tee.DefaultMaxHeaderBytes,
		ReadHeaderTimeout: tee.DefaultReadHeaderTimeout,
		ReadTimeout: tee.DefaultReadTimeout,
		WriteTimeout: tee.DefaultWriteTimeout,
		IdleTimeout: tee.DefaultIdleTimeout,
	}

	logger.Info("proxy server started", slog.String("addr", server.Addr))
	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("proxy server error", slog.String("error", err.Error()))
	}
}
