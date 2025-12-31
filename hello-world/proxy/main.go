package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/tahardi/bearclave-examples/internal/networking"
	"github.com/tahardi/bearclave-examples/internal/setup"

	"github.com/tahardi/bearclave/tee"
)

const DefaultTimeout = 5 * time.Second

func MakeAttestHandler(
	socket *tee.Socket,
	enclaveAddr string,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("reading request body", slog.String("error", err.Error()))
			tee.WriteError(w, fmt.Errorf("reading request body: %w", err))
			return
		}
		defer r.Body.Close()

		sendCtx, sendCancel := context.WithTimeout(r.Context(), DefaultTimeout)
		defer sendCancel()

		logger.Info("sending attestation request to enclave...")
		err = socket.Send(sendCtx, enclaveAddr, bodyBytes)
		if err != nil {
			logger.Error("sending attestation to enclave", slog.String("error", err.Error()))
			tee.WriteError(w, fmt.Errorf("sending attestation to enclave: %w", err))
			return
		}

		receiveCtx, receiveCancel := context.WithTimeout(r.Context(), DefaultTimeout)
		defer receiveCancel()

		logger.Info("waiting for attestation from enclave...")
		attestBytes, err := socket.Receive(receiveCtx)
		if err != nil {
			logger.Error("receiving attestation from enclave", slog.String("error", err.Error()))
			tee.WriteError(w, fmt.Errorf("receiving attestation from enclave: %w", err))
			return
		}

		attestResult := tee.AttestResult{}
		err = json.Unmarshal(attestBytes, &attestResult)
		if err != nil {
			logger.Error("unmarshaling attestation", slog.String("error", err.Error()))
			tee.WriteError(w, fmt.Errorf("unmarshaling attestation: %w", err))
			return
		}

		resp := networking.AttestUserDataResponse{Attestation: &attestResult}
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

	sockCtx, sockCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer sockCancel()
	socket, err := tee.NewSocket(
		sockCtx,
		config.Platform,
		config.Proxy.Network,
		config.Proxy.OutAddr,
	)
	if err != nil {
		logger.Error("making socket", slog.String("error", err.Error()))
		return
	}
	defer socket.Close()

	mux := http.NewServeMux()
	mux.Handle(
		"POST "+networking.AttestUserDataPath,
		MakeAttestHandler(socket, config.Enclave.Addr, logger),
	)
	servCtx, servCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer servCancel()
	server, err := tee.NewServer(
		servCtx,
		config.Platform,
		config.Proxy.Network,
		config.Proxy.InAddr,
		mux,
	)
	if err != nil {
		logger.Error("making server", slog.String("error", err.Error()))
		return
	}

	logger.Info("proxy server started", slog.String("addr", server.Addr()))
	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("proxy server error", slog.String("error", err.Error()))
	}
}
