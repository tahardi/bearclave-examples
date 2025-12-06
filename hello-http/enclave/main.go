package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"

	"bearclave-examples/internal/setup"

	"github.com/tahardi/bearclave"
	"github.com/tahardi/bearclave/tee"
)

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

	serverMux := http.NewServeMux()
	serverMux.Handle(
		"POST "+tee.AttestUserDataPath,
		tee.MakeAttestUserDataHandler(attester, logger),
	)

	server, err := tee.NewServer(
		config.Platform,
		config.Enclave.Network,
		config.Enclave.Addr,
		serverMux,
	)
	if err != nil {
		logger.Error("making server", slog.String("error", err.Error()))
		return
	}

	logger.Info("enclave server started", slog.String("addr", server.Addr()))
	err = server.Serve()
	if err != nil {
		logger.Error("enclave server error", slog.String("error", err.Error()))
	}
}
