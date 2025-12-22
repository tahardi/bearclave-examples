package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"

	"bearclave-examples/internal/networking"
	"bearclave-examples/internal/setup"

	"github.com/tahardi/bearclave/tee"
)

const (
	DefaultHost    = "127.0.0.1"
	DefaultPort    = 8080
	DefaultTimeout = 5 * time.Second
)

var (
	configFile string
	host       string
	port       int
)

func main() {
	flag.StringVar(
		&configFile,
		"config",
		"configs/nonclave/notee.yaml",
		"The Trusted Computing platform to use. Options: "+
			"nitro, sev, tdx, notee (default: notee)",
	)
	flag.StringVar(
		&host,
		"host",
		DefaultHost,
		"The hostname of the enclave proxy to connect to (default: 127.0.0.1)",
	)
	flag.IntVar(
		&port,
		"port",
		DefaultPort,
		"The port of the enclave proxy to connect to (default: 8080)",
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config, err := setup.LoadConfig(configFile)
	if err != nil {
		logger.Error("loading config", slog.String("error", err.Error()))
		return
	}
	logger.Info("loaded config", slog.Any(configFile, config))

	verifier, err := tee.NewVerifier(config.Platform)
	if err != nil {
		logger.Error("making verifier", slog.String("error", err.Error()))
		return
	}

	nonce := []byte("random nonce here")
	want := []byte("Hello, world!")
	url := "http://" + net.JoinHostPort(host, strconv.Itoa(port))
	client := networking.NewClient(url)

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	got, err := client.AttestUserData(ctx, nonce, want)
	if err != nil {
		logger.Error("attesting userdata", slog.String("error", err.Error()))
		return
	}

	measurement := config.Nonclave.Measurement
	verified, err := verifier.Verify(
		got.Attestation,
		tee.WithVerifyMeasurement(measurement),
		tee.WithVerifyNonce(nonce),
	)
	if err != nil {
		logger.Error("verifying attestation", slog.String("error", err.Error()))
		return
	}

	logger.Info(
		"attested and verified userdata",
		slog.String("userdata", string(verified.UserData)),
	)
}
