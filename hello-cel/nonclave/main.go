package main

import (
	"context"
	"encoding/json"
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
	DefaultTimeout = 15 * time.Second
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
		logger.Error("attesting expr", slog.String("error", err.Error()))
		return
	}

	attestation := got.Attestation
	measurement := config.Nonclave.Measurement
	verified, err := verifier.Verify(attestation, tee.WithVerifyMeasurement(measurement))
	if err != nil {
		logger.Error("verifying attestation", slog.String("error", err.Error()))
		return
	}
	logger.Info("verified attestation")

	attestedCEL := networking.AttestedCEL{}
	err = json.Unmarshal(verified.UserData, &attestedCEL)
	if err != nil {
		logger.Error("unmarshaling attested cel", slog.String("error", err.Error()))
		return
	}

	logger.Info(
		"attested cel",
		slog.String("expression", attestedCEL.Expression),
		slog.Any("env", attestedCEL.Env),
	)

	resultString, ok := attestedCEL.Output.(string)
	if !ok {
		logger.Error("expected string output from expression", slog.Any("got", attestedCEL.Output))
		return
	}
	logger.Info("expression result:", slog.String("value", resultString))
}
