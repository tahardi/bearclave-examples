package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/tahardi/bearclave-examples/internal/networking"
	"github.com/tahardi/bearclave-examples/internal/setup"

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

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	socket, err := tee.NewSocket(
		ctx,
		config.Platform,
		tee.NetworkTCP4,
		config.Enclave.Addr,
	)
	if err != nil {
		return fmt.Errorf("making socket: %w", err)
	}

	for {
		logger.Info("waiting to receive userdata from enclave-proxy...")
		ctx := context.Background()
		reqBytes, err := socket.Receive(ctx)
		if err != nil {
			return fmt.Errorf("receiving userdata: %w", err)
		}

		req := networking.AttestUserDataRequest{}
		err = json.Unmarshal(reqBytes, &req)
		if err != nil {
			return fmt.Errorf("unmarshaling request: %w", err)
		}

		userdata := req.UserData
		logger.Info(
			"attesting",
			slog.String("nonce", string(req.Nonce)),
			slog.String("userdata", string(userdata)),
		)
		attestResult, err := attester.Attest(
			tee.WithAttestNonce(req.Nonce),
			tee.WithAttestUserData(userdata),
		)
		if err != nil {
			return fmt.Errorf("attesting: %w", err)
		}

		attestBytes, err := json.Marshal(attestResult)
		if err != nil {
			return fmt.Errorf("marshaling attestation: %w", err)
		}

		logger.Info("sending attestation to enclave-proxy...")
		err = socket.Send(ctx, config.Proxy.Addr, attestBytes)
		if err != nil {
			return fmt.Errorf("sending attestation: %w", err)
		}
	}
}
