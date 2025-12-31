# Hello, HTTP

This example serves as an introduction to writing HTTP servers and clients
with the Bearclave SDK. For more information on networking with TEEs, and
other TEE concepts, refer to the Bearclave SDK
[TEE Overview](https://github.com/tahardi/bearclave/blob/main/docs/concepts.md).

## Walkthrough

The scenario is as follows: a Nonclave client wants an Enclave to make an HTTP
call on their behalf and to attest to the response. Since we do not have
direct access to the Enclave, we need to configure the Proxy to forward our
requests to the Enclave, as well as the Enclave's request that it makes on our
behalf.

1. The Nonclave uses our `networking.Client.AttestHTTPCall` convenience function
to send an attest HTTP request to the Enclave.

<!-- pluck("function", "main", "hello-http/nonclave/main.go", 35, 45) -->
```go
func main() {
	// ...
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	proxyURL := "http://" + net.JoinHostPort(host, strconv.Itoa(port))
	client := networking.NewClient(proxyURL)
	got, err := client.AttestHTTPCall(ctx, TargetMethod, TargetURL)
	if err != nil {
		logger.Error("attesting http call", slog.String("error", err.Error()))
		return
	}
	// ...
}
```

2. To handle our requests to the Enclave, the Proxy creates a reverse proxy
server that forwards requests to the Enclave. Recall that Nitro Enclaves have
to communicate via virtual sockets. If our `config.Platform` specifies `nitro`,
then the `tee.NewReverseProxy` function will create a reverse proxy server that
listens for incoming requests on a normal socket, but forwards them to the
Enclave via a virtual socket.

<!-- pluck("function", "main", "hello-http/proxy/main.go", 17, 33) -->
```go
func main() {
	// ...
	inCtx, inCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer inCancel()

	inboundServer, err := tee.NewReverseProxy(
		inCtx,
		config.Platform,
		config.Proxy.Network,
		config.Proxy.InAddr,
		config.Enclave.Addr,
		config.Enclave.Route,
	)
	if err != nil {
		logger.Error("making inbound server", slog.String("error", err.Error()))
		return
	}
	defer inboundServer.Close()
	// ...
}
```

3. For handling the Enclave's outbound HTTP requests, the Proxy creates an
HTTP server that uses the `tee.MakeForwardHTTPRequestHandler` function to
copy and forward requests on behalf of the Enclave. Again, Nitro requires the
use of virtual sockets. When running on Nitro, the Proxy `OutAddr` should be
set to a virtual socket address (e.g., `http://3:8082`) instead of a standard
address (e.g., `http://127.0.0.1:8082`).

<!-- pluck("function", "main", "hello-http/proxy/main.go", 34, 54) -->
```go
func main() {
	// ...
	outCtx, outCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer outCancel()

	forwardingClient := &http.Client{Timeout: DefaultTimeout}
	mux := http.NewServeMux()
	mux.HandleFunc(
		tee.ForwardPath,
		tee.MakeForwardHTTPRequestHandler(forwardingClient, logger, DefaultTimeout),
	)
	outboundServer, err := tee.NewServer(
		outCtx,
		config.Platform,
		config.Proxy.Network,
		config.Proxy.OutAddr,
		mux,
	)
	if err != nil {
		logger.Error("making outbound server", slog.String("error", err.Error()))
		return
	}
	// ...
}
```

4. To make outgoing HTTP requests, the Enclave uses `tee.NewProxiedClient` to
create an `*http.Client` that is configured to route requests to the Proxy
instead of the target URL. When running on Nitro, the client is configured to
use a virtual socket as the transport instead of a normal one.

<!-- pluck("function", "main", "hello-http/enclave/main.go", 23, 28) -->
```go
func main() {
	// ...
	client, err := tee.NewProxiedClient(config.Platform, config.Proxy.OutAddr)
	if err != nil {
		logger.Error("making proxied client", slog.String("error", err.Error()))
		return
	}
	// ...
}
```

5. The Enclave then makes an HTTP server with a handler for making HTTP calls
on behalf of a Nonclave client. When running on Nitro, `tee.NewServer` will
create a server that listens on a virtual socket instead of a normal one.
Notice how we pass the proxied client created in the previous step to the
make handler function. This is so we route calls to the Proxy instead of the
target URL.

<!-- pluck("function", "main", "hello-http/enclave/main.go", 29, 48) -->
```go
func main() {
	// ...
	serverMux := http.NewServeMux()
	serverMux.Handle(
		"POST "+networking.AttestHTTPCallPath,
		networking.MakeAttestHTTPCallHandler(DefaultTimeout, attester, client, logger),
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
	// ...
}
```

6. The `networking.MakeAttestHTTPCallHandler` takes an `*http.Client`, which we
know will route requests to the Proxy. After making the Nonclave's request, it
attests to the response and returns it to the Nonclave. The `tee.AttestResult`
struct contains both an attestation and a "user data" array, which in this case
contains the response body.

<!-- pluck("function", "MakeAttestHTTPCallHandler", "internal/networking/handlers.go", 11, 41) -->
```go
func MakeAttestHTTPCallHandler(
	ctxTimeout time.Duration,
	attester *tee.Attester,
	client *http.Client,
	logger *slog.Logger,
) http.HandlerFunc {
	// ...
		req, err := http.NewRequestWithContext(ctx, apiCallReq.Method, apiCallReq.URL, nil)
		if err != nil {
			WriteError(w, fmt.Errorf("creating request: %w", err))
			return
		}

		logger.Info(
			"making http call",
			slog.String("method", apiCallReq.Method),
			slog.String("url", apiCallReq.URL),
		)
		resp, err := client.Do(req)
		if err != nil {
			WriteError(w, fmt.Errorf("sending request: %w", err))
			return
		}
		defer resp.Body.Close()

		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			WriteError(w, fmt.Errorf("reading response body: %w", err))
			return
		}

		logger.Info("attesting http call")
		attestation, err := attester.Attest(tee.WithAttestUserData(respBytes))
		if err != nil {
			WriteError(w, fmt.Errorf("attesting: %w", err))
			return
		}
	// ...
}
```

7. Finally, the Nonclave verifies the attestation and extracts the verified
response body.

<!-- pluck("function", "main", "hello-http/nonclave/main.go", 46, 67) -->
```go
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

	httpBinResp := HTTPBinGetResponse{}
	err = json.Unmarshal(verified.UserData, &httpBinResp)
	if err != nil {
		logger.Error("unmarshaling httpbin response", slog.String("error", err.Error()))
		return
	}

	logger.Info(
		"verified http call response",
		slog.String("url", httpBinResp.URL),
		slog.Any("response", httpBinResp),
	)
}
```

## Next Steps

You know now how to write HTTP servers and clients for cloud-based TEE platforms!
Next, try out [Hello, Expr](../hello-expr/README.md) or
[Hello, CEL](../hello-cel/README.md) to learn how to make an off-chain compute
solution with TEEs.
