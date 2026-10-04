package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/tahardi/bearclave-examples/internal/networking"
	"github.com/tahardi/bearclave-examples/internal/setup"

	"github.com/tahardi/bearclave/tee"
)

const (
	DefaultHost        = "127.0.0.1"
	DefaultPort        = 8080
	DefaultVerifyDebug = false
	DefaultTimeout     = 15 * time.Second
	TargetMethod       = "GET"
	TargetURL          = "http://httpbin.org/get"
)

var (
	configFile  string
	host        string
	port        int
	verifyDebug bool
)

type HTTPBinGetResponse struct {
	Args    map[string]string `json:"args"`
	Headers map[string]string `json:"headers"`
	Origin  string            `json:"origin"`
	URL     string            `json:"url"`
}

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
	flag.BoolVar(
		&verifyDebug,
		"verify-debug",
		DefaultVerifyDebug,
		"Allow attestations from enclaves running in debug mode (default: false)",
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	err := run(logger)
	if err != nil {
		logger.Error("running nonclave", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	config, err := setup.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	logger.Info("loaded config", slog.Any(configFile, config))

	verifier, err := tee.NewVerifier(config.Platform)
	if err != nil {
		return fmt.Errorf("making verifier: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	proxyURL := "http://" + net.JoinHostPort(host, strconv.Itoa(port))
	client := networking.NewClient(proxyURL)
	got, err := client.AttestHTTPCall(ctx, TargetMethod, TargetURL)
	if err != nil {
		return fmt.Errorf("attesting http call: %w", err)
	}

	attestation := got.Attestation
	measurement := config.Nonclave.Measurement
	verified, err := verifier.Verify(
		attestation,
		tee.WithVerifyMeasurement(measurement),
		tee.WithVerifyDebug(verifyDebug),
	)
	if err != nil {
		return fmt.Errorf("verifying attestation: %w", err)
	}
	logger.Info("verified attestation")

	httpBinResp := HTTPBinGetResponse{}
	err = json.Unmarshal(verified.UserData, &httpBinResp)
	if err != nil {
		return fmt.Errorf("unmarshaling httpbin response: %w", err)
	}

	logger.Info(
		"verified http call response",
		slog.String("url", httpBinResp.URL),
		slog.Any("response", httpBinResp),
	)

	return nil
}
