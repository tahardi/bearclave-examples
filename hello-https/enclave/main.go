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

const (
	DefaultTimeout = 15 * time.Second
	DomainKey      = "domain"
)

var (
	configFile string
)

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
	defer attester.Close()

	domain, _ := config.Enclave.GetArg(DomainKey, tee.DefaultDomain).(string)
	certProvider, err := tee.NewSelfSignedCertProvider(domain, tee.DefaultIP, tee.DefaultValidity)
	if err != nil {
		return fmt.Errorf("making certProvider: %w", err)
	}

	serverMux := http.NewServeMux()
	serverMux.HandleFunc(
		networking.AttestCertPath,
		networking.MakeAttestCertHandler(attester, certProvider, logger),
	)

	serverCtx, serverCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer serverCancel()
	server, err := tee.NewServer(
		serverCtx,
		config.Platform,
		config.Enclave.Addr,
		serverMux,
		logger,
	)
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}
	defer server.Close()

	proxiedClient, err := tee.NewProxiedClient(config.Platform, config.Proxy.AddrTLS)
	if err != nil {
		return fmt.Errorf("making proxied client: %w", err)
	}

	serverTLSMux := http.NewServeMux()
	serverTLSMux.HandleFunc(
		networking.AttestHTTPSCallPath,
		networking.MakeAttestHTTPSCallHandler(DefaultTimeout, attester, proxiedClient, logger),
	)

	serverTLSCtx, serverTLSCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer serverTLSCancel()
	serverTLS, err := tee.NewServerTLS(
		serverTLSCtx,
		config.Platform,
		config.Enclave.AddrTLS,
		serverTLSMux,
		certProvider,
		logger,
	)
	if err != nil {
		return fmt.Errorf("creating serverTLS: %w", err)
	}
	defer serverTLS.Close()

	go func() {
		logger.Info("enclave server started", slog.String("addr", server.Addr()))
		err := server.Serve()
		if err != nil {
			logger.Error("enclave server error", slog.String("error", err.Error()))
		}
	}()

	logger.Info("enclave serverTLS started", slog.String("addr", serverTLS.Addr()))
	err = serverTLS.Serve()
	if err != nil {
		return fmt.Errorf("serving enclave serverTLS: %w", err)
	}

	return nil
}
