package main

import (
	"context"
	"errors"
	"flag"
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
	config, err := setup.LoadConfig(configFile)
	if err != nil {
		logger.Error("loading config", slog.String("error", err.Error()))
		return
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
		logger.Error("making revProxy server", slog.String("error", err.Error()))
		return
	}
	defer revProxy.Close()

	revTLSCtx, revTLSCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer revTLSCancel()

	revProxyTLS, err := tee.NewReverseProxyTLS(
		revTLSCtx,
		config.Platform,
		config.Proxy.RevAddrTLS,
		config.Enclave.AddrTLS,
		logger,
	)
	if err != nil {
		logger.Error("making revProxyTLS server", slog.String("error", err.Error()))
		return
	}
	defer revProxyTLS.Close()

	proxyTLSCtx, proxyTLSCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer proxyTLSCancel()

	proxyTLS, err := tee.NewProxyTLS(
		proxyTLSCtx,
		config.Platform,
		config.Proxy.AddrTLS,
		logger,
	)
	if err != nil {
		logger.Error("making proxyTLS server", slog.String("error", err.Error()))
		return
	}
	defer proxyTLS.Close()

	go func() {
		logger.Info("revProxy server started", slog.String("addr", revProxy.Addr()))
		err := revProxy.Serve()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("revProxy server error", slog.String("error", err.Error()))
		}
	}()

	go func() {
		logger.Info("revProxyTLS server started", slog.String("addr", revProxyTLS.Addr()))
		err := revProxyTLS.Serve()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("revProxyTLS server error", slog.String("error", err.Error()))
		}
	}()

	logger.Info("proxyTLS server started", slog.String("addr", proxyTLS.Addr()))
	err = proxyTLS.Serve()
	if err != nil {
		logger.Error("proxyTLS server error", slog.String("error", err.Error()))
	}
}
