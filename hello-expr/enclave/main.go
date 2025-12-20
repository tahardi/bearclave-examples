package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"bearclave-examples/internal/engine"
	"bearclave-examples/internal/networking"
	"bearclave-examples/internal/setup"

	"github.com/tahardi/bearclave/tee"
)

const DefaultTimeout = 15 * time.Second

var configFile string

func MakeHTTPGet(client *http.Client) engine.ExprEngineFn {
	return func(params ...any) (any, error) {
		if len(params) < 1 {
			return nil, errors.New("url not provided")
		}

		url, ok := params[0].(string)
		if !ok {
			return nil, errors.New("url should be a string")
		}

		resp, err := client.Get(url)
		switch {
		case err != nil:
			return nil, fmt.Errorf("making GET req to '%s': %w", url, err)
		case resp.StatusCode != http.StatusOK:
			msg := fmt.Sprintf("received non-200 response: %s", resp.Status)
			return nil, errors.New(msg)
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading body: %w", err)
		}

		var result any
		if err = json.Unmarshal(bodyBytes, &result); err != nil {
			return nil, fmt.Errorf("decoding JSON response: %w", err)
		}
		return result, nil
	}
}

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

	attester, err := tee.NewAttester(config.Platform)
	if err != nil {
		logger.Error("making attester", slog.String("error", err.Error()))
		return
	}

	client, err := tee.NewProxiedClient(config.Platform, config.Proxy.OutAddr)
	if err != nil {
		logger.Error("making proxied client", slog.String("error", err.Error()))
		return
	}

	whitelist := map[string]engine.ExprEngineFn{
		"httpGet": MakeHTTPGet(client),
	}
	exprEngine, err := engine.NewExprEngineWithWhitelist(whitelist)
	if err != nil {
		logger.Error("making expr engine", slog.String("error", err.Error()))
		return
	}

	serverMux := http.NewServeMux()
	serverMux.Handle(
		"POST "+networking.AttestExprPath,
		networking.MakeAttestExprHandler(exprEngine, DefaultTimeout, attester, logger),
	)

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	server, err := tee.NewServer(
		ctx,
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
	err = server.ListenAndServe()
	if err != nil {
		logger.Error(
			"enclave server error",
			slog.String("error", err.Error()),
		)
	}
}
