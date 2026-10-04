package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/tahardi/bearclave-examples/internal/networking"
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

	attester, err := tee.NewAttester(config.Platform)
	if err != nil {
		return fmt.Errorf("making attester: %w", err)
	}

	client, err := tee.NewProxiedClient(config.Platform, config.Proxy.Addr)
	if err != nil {
		return fmt.Errorf("making proxied client: %w", err)
	}

	serverMux := http.NewServeMux()
	serverMux.Handle(
		"POST "+networking.AttestHTTPCallPath,
		networking.MakeAttestHTTPCallHandler(DefaultTimeout, attester, client, logger),
	)

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	server, err := tee.NewServer(
		ctx,
		config.Platform,
		config.Enclave.Addr,
		serverMux,
		logger,
	)
	if err != nil {
		return fmt.Errorf("making server: %w", err)
	}

	logger.Info("enclave server started", slog.String("addr", server.Addr()))
	err = server.Serve()
	if err != nil {
		return fmt.Errorf("serving enclave server: %w", err)
	}

	return nil
}
