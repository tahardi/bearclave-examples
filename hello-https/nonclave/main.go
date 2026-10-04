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
	DefaultPortTLS     = 8443
	DefaultVerifyDebug = false
	DefaultTimeout     = 15 * time.Second
	DomainKey          = "domain"
	TargetMethod       = "GET"
	TargetURL          = "https://httpbin.org/get"
)

var (
	configFile  string
	host        string
	port        int
	portTLS     int
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
	flag.IntVar(
		&portTLS,
		"port-tls",
		DefaultPortTLS,
		"The port of the enclave TLS proxy to connect to (default: 8443)",
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

	proxyURL := "http://" + net.JoinHostPort(host, strconv.Itoa(port))
	client := networking.NewClient(proxyURL)

	certCtx, certCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer certCancel()
	attestedCert, err := client.AttestCertChain(certCtx)
	if err != nil {
		return fmt.Errorf("attesting cert: %w", err)
	}

	verifiedCert, err := verifier.Verify(
		attestedCert.Attestation,
		tee.WithVerifyMeasurement(config.Nonclave.Measurement),
		tee.WithVerifyDebug(verifyDebug),
	)
	if err != nil {
		return fmt.Errorf("verifying cert attestation: %w", err)
	}
	logger.Info("verified cert attestation")

	proxyTLSURL := "https://" + net.JoinHostPort(host, strconv.Itoa(portTLS))
	clientTLS := networking.NewClient(proxyTLSURL)
	domain, _ := config.Nonclave.GetArg(DomainKey, tee.DefaultDomain).(string)
	err = clientTLS.AddCertChain(verifiedCert.UserData, domain)
	if err != nil {
		return fmt.Errorf("adding cert: %w", err)
	}

	logger.Info("attesting https call", slog.String("revProxyTLS", proxyTLSURL))
	httpsCtx, httpsCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer httpsCancel()
	attestedCall, err := clientTLS.AttestHTTPSCall(httpsCtx, TargetMethod, TargetURL)
	if err != nil {
		return fmt.Errorf("attesting https call: %w", err)
	}

	verifiedCall, err := verifier.Verify(
		attestedCall.Attestation,
		tee.WithVerifyMeasurement(config.Nonclave.Measurement),
		tee.WithVerifyDebug(verifyDebug),
	)
	if err != nil {
		return fmt.Errorf("verifying call attestation: %w", err)
	}

	httpBinResp := HTTPBinGetResponse{}
	err = json.Unmarshal(verifiedCall.UserData, &httpBinResp)
	if err != nil {
		return fmt.Errorf("unmarshaling httpbin response: %w", err)
	}

	logger.Info(
		"verified https call response",
		slog.String("url", httpBinResp.URL),
		slog.Any("response", httpBinResp),
	)

	return nil
}
