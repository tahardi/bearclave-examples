package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
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
	TargetMethod   = "GET"
	TargetURL      = "http://httpbin.org/get"
)

var (
	configFile string
	host       string
	port       int
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
	got, err := client.AttestAPICall(ctx, TargetMethod, TargetURL)
	if err != nil {
		logger.Error("attesting api call", slog.String("error", err.Error()))
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

	hash := sha256.Sum256(got.Response)
	if !bytes.Equal(hash[:], verified.UserData) {
		logger.Error(
			"userdata verification failed",
			slog.String("want", base64.StdEncoding.EncodeToString(hash[:])),
			slog.String("got", base64.StdEncoding.EncodeToString(verified.UserData)),
		)
		return
	}

	httpBinResp := HTTPBinGetResponse{}
	err = json.Unmarshal(got.Response, &httpBinResp)
	if err != nil {
		logger.Error("unmarshaling httpbin response", slog.String("error", err.Error()))
		return
	}

	logger.Info(
		"verified api call response",
		slog.String("url", httpBinResp.URL),
		slog.Any("response", httpBinResp),
	)
}
