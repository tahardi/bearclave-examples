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

	"github.com/tahardi/bearclave-examples/internal/engine"
	"github.com/tahardi/bearclave-examples/internal/networking"
	"github.com/tahardi/bearclave-examples/internal/setup"

	"github.com/tahardi/bearclave/tee"
)

const DefaultTimeout = 15 * time.Second

var (
	ErrHTTPGet               = errors.New("http get")
	ErrHTTPGetMissingURL     = fmt.Errorf("%w: missing url", ErrHTTPGet)
	ErrHTTPGetWrongURLType   = fmt.Errorf("%w: url should be a string", ErrHTTPGet)
	ErrHTTPGetNon200Response = fmt.Errorf("%w: non-200 response", ErrHTTPGet)

	configFile string
)

func MakeHTTPGet(client *http.Client) engine.ExprEngineFn {
	return func(params ...any) (any, error) {
		if len(params) < 1 {
			return nil, ErrHTTPGetMissingURL
		}

		url, ok := params[0].(string)
		if !ok {
			return nil, ErrHTTPGetWrongURLType
		}

		ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating GET req: %w", err)
		}

		resp, err := client.Do(req)
		switch {
		case err != nil:
			return nil, fmt.Errorf("making GET req to '%s': %w", url, err)
		case resp.StatusCode != http.StatusOK:
			return nil, fmt.Errorf("%w: %s", ErrHTTPGetNon200Response, resp.Status)
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
		config.Enclave.Addr,
		serverMux,
		logger,
	)
	if err != nil {
		logger.Error("making server", slog.String("error", err.Error()))
		return
	}

	logger.Info("enclave server started", slog.String("addr", server.Addr()))
	err = server.Serve()
	if err != nil {
		logger.Error(
			"enclave server error",
			slog.String("error", err.Error()),
		)
	}
}
