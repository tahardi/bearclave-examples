# Hello, Expr

[Expr](https://github.com/expr-lang/expr) is a fast, memory-safe, intuitive
expression evaluator optimized for the Go language. This example demonstrates
how to build a Bearclave server that runs client-provided expressions in a
secure TEE environment. Try it out yourself! Run the example locally with:

```bash
make

# You should see output similar to:
[proxy  ] time=2025-12-20T14:46:03.169-05:00 level=INFO msg="loaded config" configs/enclave/notee.yaml="&{Platform:notee Enclave:{Network:tcp Addr:http://127.0.0.1:8083 Route:app/v1} Nonclave:{Measurement: Route:} Proxy:{Network:tcp InAddr:http://0.0.0.0:8080 OutAddr:http://127.0.0.1:8082}}"
[proxy  ] time=2025-12-20T14:46:03.170-05:00 level=INFO msg="proxy outbound server started"
[proxy  ] time=2025-12-20T14:46:03.170-05:00 level=INFO msg="proxy inbound server started"
[enclave        ] time=2025-12-20T14:46:03.719-05:00 level=INFO msg="loaded config" configs/enclave/notee.yaml="&{Platform:notee Enclave:{Network:tcp Addr:http://127.0.0.1:8083 Route:app/v1} Nonclave:{Measurement: Route:} Proxy:{Network:tcp InAddr:http://0.0.0.0:8080 OutAddr:http://127.0.0.1:8082}}"
[enclave        ] time=2025-12-20T14:46:03.719-05:00 level=INFO msg="enclave server started" addr=127.0.0.1:8083
[nonclave       ] time=2025-12-20T14:46:04.220-05:00 level=INFO msg="loaded config" configs/nonclave/notee.yaml="&{Platform:notee Enclave:{Network: Addr: Route:} Nonclave:{Measurement:Not a TEE platform. Code measurements are not real. Route:app/v1} Proxy:{Network: InAddr: OutAddr:}}"
[enclave        ] time=2025-12-20T14:46:04.221-05:00 level=INFO msg="executing expr" expression="httpGet(targetUrl).url == targetUrl ? \"URL Match Success\" : \"URL Mismatch\""
[proxy  ] time=2025-12-20T14:46:04.221-05:00 level=INFO msg="forwarding request" url=http://httpbin.org/get
[enclave        ] time=2025-12-20T14:46:04.496-05:00 level=INFO msg="attesting expr" hash="NTP1x0ckuRyhi9bs7EWP/a/bQ5nF85Av1FJUhCy4LIs="
[nonclave       ] time=2025-12-20T14:46:04.497-05:00 level=INFO msg="verified attestation"
[nonclave       ] time=2025-12-20T14:46:04.497-05:00 level=INFO msg="verified expression" expression="httpGet(targetUrl).url == targetUrl ? \"URL Match Success\" : \"URL Mismatch\"" env=map[targetUrl:http://httpbin.org/get]
[nonclave       ] time=2025-12-20T14:46:04.497-05:00 level=INFO msg="expression result:" value="URL Match Success"
```

## How it Works

1. The Client defines an expression and a set of environment variables. In this
example, the Client wants to fetch some data from a remote server and verify
that the URL matches the expected value.
<!-- pluck("function", "main", "hello-expr/nonclave/main.go", 41, 50) -->
```go
func main() {
	// ...
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	proxyURL := "http://" + net.JoinHostPort(host, strconv.Itoa(port))
	client := networking.NewClient(proxyURL)

	env := map[string]any{
		"targetUrl": "http://httpbin.org/get",
	}
	// ...
}
```

2. The Expr runtime provides a limited set of builtin functions by default to
ensure expressions are tightly sandboxed. We can provide additional functionality
by passing in custom functions, however. In this example, the Enclave defines
an `httpGet` function that allows expressions to make basic HTTP GET requests.
<!-- pluck("function", "MakeHTTPGet", "hello-expr/enclave/main.go", 0, 0) -->
```go
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
```

3. Further down we see how the Enclave registers the `httpGet` function with the
Expr engine.
<!-- pluck("function", "main", "hello-expr/enclave/main.go", 23, 43) -->
```go
func main() {
	// ...
	client, err := tee.NewProxiedClient(config.Platform, config.Proxy.Addr)
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
	// ...
}
```

4. Let's break down the `AttestExprHandler` function. The Enclave expects the
Client to send the expression string and any necessary environment variables.
<!-- pluck("type", "AttestExprRequest", "internal/networking/handlers.go", 0, 0) -->
```go
type AttestExprRequest struct {
	Expression string         `json:"expression"`
	Env        map[string]any `json:"env"`
}
```

5. In return, the Enclave will execute the expression and attest to the output.
Notice how everything---the expression, the input, and the output---is included
in the attestation. This way, the Client is assured that the output is both
genuine (i.e., produced by the Enclave) and correct (i.e., generated by the
specified expression and inputs).
<!-- pluck("type", "AttestedExpr", "internal/networking/handlers.go", 0, 0) -->
```go
type AttestedExpr struct {
	Expression string `json:"expression"`
	Env        any    `json:"env"`
	Output     any    `json:"output"`
}
```

<!-- pluck("function", "MakeAttestExprHandler", "internal/networking/handlers.go", 9, 39) -->
```go
func MakeAttestExprHandler(
	exprEngine *engine.ExprEngine,
	exprTimeout time.Duration,
	attester *tee.Attester,
	logger *slog.Logger,
) http.HandlerFunc {
	// ...

		ctx, cancel := context.WithTimeout(r.Context(), exprTimeout)
		defer cancel()

		logger.Info("executing expr", slog.String("expression", exprReq.Expression))
		output, err := exprEngine.Execute(ctx, exprReq.Expression, exprReq.Env)
		if err != nil {
			logger.Error("executing expression", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("executing expression: %w", err))
			return
		}

		result := AttestedExpr{
			Expression: exprReq.Expression,
			Env:        exprReq.Env,
			Output:     output,
		}
		resBytes, err := json.Marshal(result)
		if err != nil {
			logger.Error("marshaling result", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("marshaling result: %w", err))
			return
		}

		logger.Info("attesting expr", slog.Any("result", result))
		attestation, err := attester.Attest(tee.WithAttestUserData(resBytes))
		if err != nil {
			logger.Error("attesting", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("attesting: %w", err))
			return
	// ...
}
```

6. A nice property of the Expr language is that all expressions are guaranteed
to terminate. User defined functions, such as our `httpGet` function, are not
though. Thus, we wrap expression executions with a context so that the
Enclave does not block forever.
<!-- pluck("function", "ExprEngine.Execute", "internal/engine/expr.go", 0, 0) -->
```go
func (e *ExprEngine) Execute(
	ctx context.Context,
	expression string,
	env map[string]any,
) (any, error) {
	program, err := expr.Compile(expression, append(e.baseOptions, expr.Env(env))...)
	if err != nil {
		return nil, fmt.Errorf("compile error: %w", err)
	}

	resultChan := make(chan any, 1)
	errChan := make(chan error, 1)
	go func() {
		output, err := expr.Run(program, env)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- output
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errChan:
		return nil, fmt.Errorf("running expr: %w", err)
	case res := <-resultChan:
		return res, nil
	}
}
```

7. When the Client receives the Enclave's response, it first verifies the
attestation.
<!-- pluck("function", "main", "hello-expr/nonclave/main.go", 51, 59) -->
```go
func main() {
	// ...
	got, err := client.AttestExpr(ctx, expression, env)
	if err != nil {
		logger.Error("attesting expr", slog.String("error", err.Error()))
		return
	}

	attestation := got.Attestation
	measurement := config.Nonclave.Measurement
	// ...
}
```

8. If the attestation successfully verifies, then the Client can extract and
use the expression result knowing that it is authentic and correct.
<!-- pluck("type", "AttestedExpr", "internal/networking/handlers.go", 0, 0) -->
```go
type AttestedExpr struct {
	Expression string `json:"expression"`
	Env        any    `json:"env"`
	Output     any    `json:"output"`
}
```

<!-- pluck("function", "main", "hello-expr/nonclave/main.go", 60, 79) -->
```go
func main() {
	// ...
		attestation,
		tee.WithVerifyMeasurement(measurement),
		tee.WithVerifyDebug(verifyDebug),
	)
	if err != nil {
		logger.Error("verifying attestation", slog.String("error", err.Error()))
		return
	}
	logger.Info("verified attestation")

	attestedExpr := networking.AttestedExpr{}
	err = json.Unmarshal(verified.UserData, &attestedExpr)
	if err != nil {
		logger.Error("unmarshaling attested expression", slog.String("error", err.Error()))
		return
	}

	logger.Info(
		"attested expression",
	// ...
}
```

## Next Steps

You now know how to execute arbitrary Client expressions in a secure TEE
environment! Check out the [Expr](https://expr-lang.org/docs/getting-started)
documentation for more information on how to use Expr in your own applications.
