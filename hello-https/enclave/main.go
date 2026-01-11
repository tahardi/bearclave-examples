package main

import (
	"context"
	"flag"
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
	DefaultDomain  = "127.0.0.1"
	Validity       = 1 * time.Hour
)

var (
	configFile string
	domain     string
)

func main() {
	flag.StringVar(
		&configFile,
		"config",
		"configs/enclave/notee.yaml",
		"The Trusted Computing platform to use. Options: "+
			"nitro, sev, tdx, notee (default: notee)",
	)
	flag.StringVar(
		&domain,
		"domain",
		DefaultDomain,
		"The domain name of the server. Used for self-signed certs.",
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config, err := setup.LoadConfig(configFile)
	if err != nil {
		logger.Error("loading config", slog.Any("error", err))
		return
	}
	logger.Info("loaded config", slog.Any(configFile, config))

	attester, err := tee.NewAttester(config.Platform)
	if err != nil {
		logger.Error("making attester", slog.String("error", err.Error()))
		return
	}
	defer attester.Close()

	certProvider, err := tee.NewSelfSignedCertProvider(domain, Validity)
	if err != nil {
		logger.Error("making certProvider", slog.String("error", err.Error()))
		return
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
		logger.Error("creating server", slog.String("error", err.Error()))
		return
	}
	defer server.Close()

	proxiedClient, err := tee.NewProxiedClient(config.Platform, config.Proxy.AddrTLS)
	if err != nil {
		logger.Error("making proxied client", slog.String("error", err.Error()))
		return
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
		logger.Error("creating serverTLS", slog.String("error", err.Error()))
		return
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
		logger.Error("enclave serverTLS error", slog.String("error", err.Error()))
	}
}
