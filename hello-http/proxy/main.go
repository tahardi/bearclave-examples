package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/tahardi/bearclave-examples/internal/setup"

	"github.com/tahardi/bearclave/tee"
)

const DefaultTimeout = 15 * time.Second

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
	err := run(logger)
	if err != nil {
		logger.Error("running", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	config, err := setup.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	logger.Info("loaded config", slog.Any(configFile, config))

	revCtx, revCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer revCancel()

	revProxy, err := tee.NewReverseProxy(
		revCtx,
		config.Platform,
		config.Proxy.RevAddr,
		config.Enclave.Addr,
		logger,
	)
	if err != nil {
		return fmt.Errorf("making inbound server: %w", err)
	}
	defer revProxy.Close()

	proxyCtx, proxyCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer proxyCancel()

	forwardingClient := &http.Client{Timeout: DefaultTimeout}
	proxy, err := tee.NewProxy(
		proxyCtx,
		config.Platform,
		config.Proxy.Addr,
		forwardingClient,
		logger,
	)
	if err != nil {
		return fmt.Errorf("making outbound server: %w", err)
	}
	defer proxy.Close()

	go func() {
		logger.Info("proxy inbound server started")
		err := revProxy.Serve()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("inbound server error", slog.String("error", err.Error()))
		}
	}()

	logger.Info("proxy outbound server started")
	err = proxy.Serve()
	if err != nil {
		return fmt.Errorf("serving outbound server: %w", err)
	}

	return nil
}
