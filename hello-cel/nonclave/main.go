package main

import (
	"context"
	"encoding/json"
	"errors"
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
	DefaultTimeout     = 15 * time.Second
	DefaultVerifyDebug = false
)

var ErrExpectedStringOutput = errors.New("expected string output from expression")

var (
	configFile  string
	host        string
	port        int
	verifyDebug bool
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

	env := map[string]any{
		"targetUrl": "http://httpbin.org/get",
	}
	expression := `httpGet(targetUrl).url == targetUrl ? "URL Match Success" : "URL Mismatch"`
	got, err := client.AttestCEL(ctx, expression, env)
	if err != nil {
		return fmt.Errorf("attesting expr: %w", err)
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

	attestedCEL := networking.AttestedCEL{}
	err = json.Unmarshal(verified.UserData, &attestedCEL)
	if err != nil {
		return fmt.Errorf("unmarshaling attested cel: %w", err)
	}

	logger.Info(
		"attested cel",
		slog.String("expression", attestedCEL.Expression),
		slog.Any("env", attestedCEL.Env),
	)

	resultString, ok := attestedCEL.Output.(string)
	if !ok {
		return fmt.Errorf("%w: got %v", ErrExpectedStringOutput, attestedCEL.Output)
	}
	logger.Info("expression result:", slog.String("value", resultString))

	return nil
}
