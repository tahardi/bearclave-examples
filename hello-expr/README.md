# Hello, Expr

[Expr](https://github.com/expr-lang/expr) is a fast, memory-safe, intuitive
expression evaluator optimized for the Go language. This example demonstrates
how to build a Bearclave server that runs client-provided expressions in a
secure TEE environment. Try it out yourself! Run the example locally with:

```bash
make

# You should see output similar to:
[proxy  ] time=2025-12-20T06:44:01.104-05:00 level=INFO msg="loaded config" configs/enclave/notee.yaml="&{Platform:notee Enclave:{Network:tcp Addr:http://127.0.0.1:8083 Route:app/v1} Nonclave:{Measurement: Route:} Proxy:{Network:tcp InAddr:http://0.0.0.0:8080 OutAddr:http://127.0.0.1:8082}}"
[proxy  ] time=2025-12-20T06:44:01.104-05:00 level=INFO msg="proxy outbound server started"
[proxy  ] time=2025-12-20T06:44:01.104-05:00 level=INFO msg="proxy inbound server started"
[enclave        ] time=2025-12-20T06:44:01.180-05:00 level=INFO msg="loaded config" configs/enclave/notee.yaml="&{Platform:notee Enclave:{Network:tcp Addr:http://127.0.0.1:8083 Route:app/v1} Nonclave:{Measurement: Route:} Proxy:{Network:tcp InAddr:http://0.0.0.0:8080 OutAddr:http://127.0.0.1:8082}}"
[enclave        ] time=2025-12-20T06:44:01.180-05:00 level=INFO msg="enclave server started" addr=127.0.0.1:8083
[nonclave       ] time=2025-12-20T06:44:01.589-05:00 level=INFO msg="loaded config" configs/nonclave/notee.yaml="&{Platform:notee Enclave:{Network: Addr: Route:} Nonclave:{Measurement:Not a TEE platform. Code measurements are not real. Route:app/v1} Proxy:{Network: InAddr: OutAddr:}}"
[proxy  ] time=2025-12-20T06:44:01.590-05:00 level=INFO msg="forwarding request" url=http://httpbin.org/get
[enclave        ] time=2025-12-20T06:44:01.710-05:00 level=INFO msg="attesting expr" hash="NTP1x0ckuRyhi9bs7EWP/a/bQ5nF85Av1FJUhCy4LIs="
[nonclave       ] time=2025-12-20T06:44:01.711-05:00 level=INFO msg="verified attestation"
[nonclave       ] time=2025-12-20T06:44:01.711-05:00 level=INFO msg="verified expression" expression="httpGet(targetUrl).url == targetUrl ? \"URL Match Success\" : \"URL Mismatch\"" env=map[targetUrl:http://httpbin.org/get]
[nonclave       ] time=2025-12-20T06:44:01.711-05:00 level=INFO msg="expression result:" value="URL Match Success"
```

## How it Works

1. The Client defines an expression and a set of environment variables. In this
example, the Client wants to fetch some data from a remote server and verify
that the URL matches the expected value.
```go
// nonclave/main.go
env := map[string]interface{}{
    "targetUrl": "http://httpbin.org/get",
}
expression := `httpGet(targetUrl).url == targetUrl ? "URL Match Success" : "URL Mismatch"`
got, err := client.AttestExprCall(ctx, expression, env)
if err != nil {
    logger.Error("attesting expr", slog.String("error", err.Error()))
    return
}
```

2. The Expr runtime provides a limited set of builtin functions by default to
ensure expressions are tightly sandboxed. We can provide additional functionality
by passing in custom functions, however. In this example, the Enclave defines
an `httpGet` function that allows expressions to make basic HTTP GET requests.
```go
// enclave/main.go
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

```

3. Further down we see how the Enclave registers the `httpGet` function with the
Expr engine.
```go
// enclave/main.go
func main() {
	// ...
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
	// ...
}
```

4. Let's breakdown the `AttestExprHandler` function. The Enclave expects the
Client to send the expression string and input variables. In return, the
Enclave will execute the expression and attest to the output.
```go
// internal/networking/handlers.go
type AttestExprRequest struct {
	Expression string         `json:"expression"`
	Env        map[string]any `json:"env"`
}

type AttestedExpr struct {
	Expression string `json:"expression"`
	Env        any    `json:"env"`
	Output     any    `json:"output"`
}

type AttestExprResponse struct {
	Attestation *tee.AttestResult `json:"attestation"`
	Result      AttestedExpr      `json:"result"`
}

func MakeAttestExprHandler(
	exprEngine *engine.ExprEngine,
	exprTimeout time.Duration,
	attester tee.Attester,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		exprReq := AttestExprRequest{}
		err := json.NewDecoder(r.Body).Decode(&exprReq)
		if err != nil {
			logger.Error("decoding request", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("decoding request: %w", err))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), exprTimeout)
		defer cancel()

		output, err := exprEngine.Execute(ctx, exprReq.Expression, exprReq.Env)
		if err != nil {
			logger.Error("evaluating expression", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("evaluating expression: %w", err))
			return
		}
		// ...
```

5. A nice property of the Expr language is that all expressions are guaranteed
to terminate. User defined functions, such as our `httpGet` function, are not
though. Thus, we wrap the call with a context so that the Enclave does not
block forever.
```go
// internal/engine/expr.go
func (e *ExprEngine) Execute(
	ctx context.Context,
	expression string,
	env map[string]any,
) (any, error) {
	whitelistedFns := []expr.Option{expr.Env(env)}
	for name, fn := range e.whitelist {
		whitelistedFns = append(whitelistedFns, expr.Function(name, fn))
	}

	program, err := expr.Compile(expression, whitelistedFns...)
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

6. Attestations are used to prove to the Client that the Enclave is running the
expected code on a genuine TEE platform. In addition, they often support the
inclusion of "user data" as a way to prove that the Enclave witnessed or
performed some event. Since user data fields are typically size limited (64 to
1024 bytes), the Enclave uses a hash of the combined inputs, expression, and
output.
```go
// internal/networking/handlers.go
func MakeAttestExprHandler(
	exprEngine *engine.ExprEngine,
	exprTimeout time.Duration,
	attester tee.Attester,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
        // ...
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

		hash := sha256.Sum256(resBytes)
		logger.Info(
			"attesting expr",
			slog.String("hash", base64.StdEncoding.EncodeToString(hash[:])),
		)
		attestation, err := attester.Attest(tee.WithAttestUserData(hash[:]))
		if err != nil {
			logger.Error("attesting", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("attesting: %w", err))
			return
		}

		apiCallResp := AttestExprResponse{
			Attestation: attestation,
			Result:      result,
		}
		WriteResponse(w, apiCallResp)
	}
}
```

7. When the Client receives the Enclave's response, it first verifies the
attestation. Again, this tells the Client that the Enclave is running the
expected code on the expected TEE platform.
```go
// nonclave/main.go
func main() {
	// ...
	attestation := got.Attestation
	measurement := config.Nonclave.Measurement
	verified, err := verifier.Verify(attestation, tee.WithVerifyMeasurement(measurement))
	if err != nil {
	logger.Error("verifying attestation", slog.String("error", err.Error()))
	return
	}
	logger.Info("verified attestation")
}
```

8. Once the Client has verified the Enclave's integrity, it must do the same
for the expression output. Remember, the output is returned *alongside* the
attestation, so there is a possibility that it has been tampered with.
Now that we know the attestation is genuine, we can extract the expected output
hash from the "user data" field and compare it to the hash of the output we
received.
```go
// internal/networking/handlers.go
type AttestedExpr struct {
    Expression string `json:"expression"`
    Env        any    `json:"env"`
    Output     any    `json:"output"`
}

type AttestExprResponse struct {
    Attestation *tee.AttestResult `json:"attestation"`
    Result      AttestedExpr      `json:"result"`
}

// nonclave/main.go
func main() {
	// ...
	resBytes, err := json.Marshal(got.Result)
	if err != nil {
	logger.Error("marshaling result", slog.String("error", err.Error()))
	return
	}

	hash := sha256.Sum256(resBytes)
    if !bytes.Equal(hash[:], verified.UserData[:len(hash)]) {
        logger.Error(
            "userdata verification failed",
            slog.String("want", base64.StdEncoding.EncodeToString(hash[:])),
            slog.String("got", base64.StdEncoding.EncodeToString(verified.UserData[:len(hash)])),
        )
        return
    }
    // ...
}
```

9. If the output hash matches the expected hash, the Client can safely extract
and use the result of the expression. Not only is the Client able to verify
the output, but any third-party can too (if the Client provides them with the
attestation).
```go
// nonclave/main.go
func main() {
	// ...
    logger.Info(
        "verified expression",
        slog.String("expression", got.Result.Expression),
        slog.Any("env", got.Result.Env),
    )
    
    resultString, ok := got.Result.Output.(string)
    if !ok {
        logger.Error("expected string output from expression", slog.Any("got", got.Result.Output))
        return
    }
    logger.Info("expression result:", slog.String("value", resultString))
}
```

## Next Steps

You now know how to execute arbitrary Client expressions in a secure TEE
environment! Check out the [Expr](https://expr-lang.org/docs/getting-started)
documentation for more information on how to use Expr in your own applications.
