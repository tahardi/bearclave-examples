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

func MakeAttestUserDataHandler(
	socket *tee.Socket,
	enclaveAddr string,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := tee.AttestUserDataRequest{}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			tee.WriteError(w, fmt.Errorf("decoding request: %w", err))
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		logger.Info("sending userdata to enclave...", slog.String("userdata", string(req.Data)))
		err = socket.Send(ctx, enclaveAddr, req.Data)
		if err != nil {
			tee.WriteError(w, fmt.Errorf("sending userdata to enclave: %w", err))
			return
		}

		logger.Info("waiting for attestation from enclave...")
		attestBytes, err := socket.Receive(ctx)
		if err != nil {
			tee.WriteError(w, fmt.Errorf("receiving attestation from enclave: %w", err))
			return
		}

		attestResult := bearclave.AttestResult{}
		err = json.Unmarshal(attestBytes, &attestResult)
		if err != nil {
			tee.WriteError(w, fmt.Errorf("unmarshaling attestation: %w", err))
			return
		}

		resp := tee.AttestUserDataResponse{Attestation: &attestResult}
		tee.WriteResponse(w, resp)
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
	}

	logger.Info("proxy server started", slog.String("addr", server.Addr))
	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("proxy server error", slog.String("error", err.Error()))
	}
}
