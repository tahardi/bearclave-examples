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

	proxyURL := "http://" + net.JoinHostPort(host, strconv.Itoa(port))
	client := networking.NewClient(proxyURL)

	certCtx, certCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer certCancel()
	attestedCert, err := client.AttestCertChain(certCtx)
	if err != nil {
		logger.Error("attesting cert", slog.String("error", err.Error()))
		return
	}

	verifiedCert, err := verifier.Verify(
		attestedCert.Attestation,
		tee.WithVerifyMeasurement(config.Nonclave.Measurement),
		tee.WithVerifyDebug(verifyDebug),
	)
	if err != nil {
		logger.Error("verifying cert attestation", slog.String("error", err.Error()))
		return
	}
	logger.Info("verified cert attestation")

	proxyTLSURL := "https://" + net.JoinHostPort(host, strconv.Itoa(portTLS))
	clientTLS := networking.NewClient(proxyTLSURL)
	enclaveDomain := config.Nonclave.GetArg(DomainKey, tee.DefaultDomain).(string)
	err = clientTLS.AddCertChain(verifiedCert.UserData, enclaveDomain)
	if err != nil {
		logger.Error("adding cert", slog.String("error", err.Error()))
		return
	}

	logger.Info("attesting https call", slog.String("revProxyTLS", proxyTLSURL))
	httpsCtx, httpsCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer httpsCancel()
	attestedCall, err := clientTLS.AttestHTTPSCall(httpsCtx, TargetMethod, TargetURL)
	if err != nil {
		logger.Error("attesting https call", slog.String("error", err.Error()))
		return
	}

	verifiedCall, err := verifier.Verify(
		attestedCall.Attestation,
		tee.WithVerifyMeasurement(config.Nonclave.Measurement),
		tee.WithVerifyDebug(verifyDebug),
	)
	if err != nil {
		logger.Error("verifying call attestation", slog.String("error", err.Error()))
		return
	}

	httpBinResp := HTTPBinGetResponse{}
	err = json.Unmarshal(verifiedCall.UserData, &httpBinResp)
	if err != nil {
		logger.Error("unmarshaling httpbin response", slog.String("error", err.Error()))
		return
	}

	logger.Info(
		"verified https call response",
		slog.String("url", httpBinResp.URL),
		slog.Any("response", httpBinResp),
	)
}
