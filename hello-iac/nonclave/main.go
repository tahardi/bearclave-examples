package main

import (
	"context"
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

const script = `
let regions = ["us-east-1", "us-west-2"]
let buckets = regions.map(r => ({type: "aws_s3_bucket", name: "bearclave-logs-" + r, region: r}))
let roles = [{type: "aws_iam_role", name: "bearclave-log-writer", region: "global"}]
[...buckets, ...roles]
`

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
	if err := run(logger); err != nil {
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

	got, err := client.AttestIaC(ctx, script)
	if err != nil {
		return fmt.Errorf("attesting iac: %w", err)
	}

	measurement := config.Nonclave.Measurement
	verified, err := verifier.Verify(
		got.Attestation,
		tee.WithVerifyMeasurement(measurement),
		tee.WithVerifyDebug(verifyDebug),
	)
	if err != nil {
		return fmt.Errorf("verifying attestation: %w", err)
	}
	logger.Info("verified attestation")

	err = networking.VerifyIaCPlan(verified.UserData, script, got.Plan)
	if err != nil {
		return fmt.Errorf("verifying plan: %w", err)
	}
	logger.Info("verified plan", slog.String("plan", string(got.Plan)))
	return nil
}
