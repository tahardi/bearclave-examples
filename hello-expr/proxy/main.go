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

	inCtx, inCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer inCancel()
	inboundServer, err := tee.NewReverseProxy(
		inCtx,
		config.Platform,
		config.Proxy.Network,
		config.Proxy.InAddr,
		config.Enclave.Addr,
		config.Enclave.Route,
	)
	if err != nil {
		logger.Error("making inbound server", slog.String("error", err.Error()))
		return
	}
	defer inboundServer.Close()

	outCtx, outCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer outCancel()

	forwardingClient := &http.Client{Timeout: DefaultTimeout}
	mux := http.NewServeMux()
	mux.HandleFunc(
		tee.ForwardPath,
		tee.MakeForwardHTTPRequestHandler(forwardingClient, logger, DefaultTimeout),
	)
	outboundServer, err := tee.NewServer(
		outCtx,
		config.Platform,
		config.Proxy.Network,
		config.Proxy.OutAddr,
		mux,
	)
	if err != nil {
		logger.Error("making outbound server", slog.String("error", err.Error()))
		return
	}
	defer outboundServer.Close()

	go func() {
		logger.Info("proxy inbound server started")
		err := inboundServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("inbound server error", slog.String("error", err.Error()))
		}
	}()

	logger.Info("proxy outbound server started")
	err = outboundServer.ListenAndServe()
	if err != nil {
		logger.Error("outbound server error", slog.String("error", err.Error()))
	}
}
