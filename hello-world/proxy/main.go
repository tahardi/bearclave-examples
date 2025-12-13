package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"bearclave-examples/internal/setup"

	"github.com/tahardi/bearclave"
	"github.com/tahardi/bearclave/tee"
)

const (
	Megabyte = 1 << 20
	DefaultRequestTimeout = 5 * time.Second
	DefaultReadHeaderTimeout = 10 * time.Second
	DefaultReadTimeout       = 15 * time.Second
	DefaultWriteTimeout      = 15 * time.Second
	DefaultIdleTimeout       = 60 * time.Second
	DefaultMaxHeaderBytes    = 1 * Megabyte // 1MB

)

func MakeAttestUserDataHandler(
	socket *tee.Socket,
	enclaveAddr string,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := tee.AttestUserDataRequest{}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			logger.Error("decoding request", slog.String("error", err.Error()))
			tee.WriteError(w, fmt.Errorf("decoding request: %w", err))
			return
		}

		sendCtx, sendCancel := context.WithTimeout(r.Context(), DefaultRequestTimeout)
		defer sendCancel()

		logger.Info("sending userdata to enclave...", slog.String("userdata", string(req.Data)))
		err = socket.Send(sendCtx, enclaveAddr, req.Data)
		if err != nil {
			logger.Error("sending userdata to enclave", slog.String("error", err.Error()))
			tee.WriteError(w, fmt.Errorf("sending userdata to enclave: %w", err))
			return
		}

		receiveCtx, receiveCancel := context.WithTimeout(r.Context(), DefaultRequestTimeout)
		defer receiveCancel()

		logger.Info("waiting for attestation from enclave...")
		attestBytes, err := socket.Receive(receiveCtx)
		if err != nil {
			logger.Error("receiving attestation from enclave", slog.String("error", err.Error()))
			tee.WriteError(w, fmt.Errorf("receiving attestation from enclave: %w", err))
			return
		}

		attestResult := bearclave.AttestResult{}
		err = json.Unmarshal(attestBytes, &attestResult)
		if err != nil {
			logger.Error("unmarshaling attestation", slog.String("error", err.Error()))
			tee.WriteError(w, fmt.Errorf("unmarshaling attestation: %w", err))
			return
		}

		resp := tee.AttestUserDataResponse{Attestation: &attestResult}
		tee.WriteResponse(w, resp)
		logger.Info("sent attestation to client")
	}
}

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

	socket, err := tee.NewSocket(config.Platform, config.Proxy.Network, config.Proxy.Addr)
	if err != nil {
		logger.Error("making socket", slog.String("error", err.Error()))
		return
	}

	serverMux := http.NewServeMux()
	serverMux.Handle(
		"POST "+tee.AttestUserDataPath,
		MakeAttestUserDataHandler(socket, config.Enclave.Addr, logger),
	)

	server := &http.Server{
		Addr:    "0.0.0.0:8080",
		Handler: serverMux,
		MaxHeaderBytes: DefaultMaxHeaderBytes,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		ReadTimeout: DefaultReadTimeout,
		WriteTimeout: DefaultWriteTimeout,
		IdleTimeout: DefaultIdleTimeout,
	}

	logger.Info("proxy server started", slog.String("addr", server.Addr))
	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("proxy server error", slog.String("error", err.Error()))
	}
}
