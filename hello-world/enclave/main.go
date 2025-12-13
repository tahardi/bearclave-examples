package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"time"

	"bearclave-examples/internal/setup"

	"github.com/tahardi/bearclave"
	"github.com/tahardi/bearclave/tee"
)

const DefaultTimeout = 5 * time.Second

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
		logger.Error("loading config", slog.Any("error", err))
		return
	}
	logger.Info("loaded config", slog.Any(configFile, config))

	attester, err := bearclave.NewAttester(config.Platform)
	if err != nil {
		logger.Error("making attester", slog.String("error", err.Error()))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	socket, err := tee.NewSocket(
		ctx,
		config.Platform,
		config.Enclave.Network,
		config.Enclave.Addr,
	)
	if err != nil {
		logger.Error("making socket", slog.String("error", err.Error()))
		return
	}

	for {
		logger.Info("waiting to receive userdata from enclave-proxy...")
		ctx := context.Background()
		userdata, err := socket.Receive(ctx)
		if err != nil {
			logger.Error("receiving userdata", slog.String("error", err.Error()))
			return
		}

		logger.Info("attesting userdata", slog.String("userdata", string(userdata)))
		attestResult, err := attester.Attest(bearclave.WithAttestUserData(userdata))
		if err != nil {
			logger.Error("attesting userdata", slog.String("error", err.Error()))
			return
		}

		attestBytes, err := json.Marshal(attestResult)
		if err != nil {
			logger.Error("marshaling attestation", slog.String("error", err.Error()))
			return
		}

		logger.Info("sending attestation to enclave-proxy...")
		err = socket.Send(ctx, config.Proxy.Addr, attestBytes)
		if err != nil {
			logger.Error("sending attestation", slog.String("error", err.Error()))
			return
		}
	}
}
