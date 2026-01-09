# Hello, HTTPS

This example serves as an introduction to writing secure HTTPS servers and clients
with the Bearclave SDK. For more information on networking with TEEs, and
other TEE concepts, refer to the Bearclave SDK
[TEE Overview](https://github.com/tahardi/bearclave/blob/main/docs/concepts.md).

## Walkthrough

The scenario is as follows: a Nonclave client wants an Enclave to make an HTTPS
call on their behalf and to attest to the response. Recall that we do not have
direct access to the Enclave, so we need to configure a Proxy to forward
requests to the Enclave, as well as the Enclave's request that it makes on
behalf of the Nonclave. Additionally, we are going to assume (for now) that
the Enclave does not have access to a Certificate Authority, but it can generate
self-signed certs. The Nonclave must first fetch an attested certificate from
the Enclave and then use it to secure future communication with the Enclave.

In summary, we will need:
- A Nonclave client that makes HTTP and HTTPS requests to an Enclave
- A Reverse Proxy that forwards HTTP requests to the Enclave
- A Reverse TLS Proxy that forwards HTTPS requests to the Enclave
- A TLS Proxy that forwards HTTPS requests from the Enclave to a remote server
- An Enclave that generates self-signed certificates and makes HTTPS requests
on behalf of the Nonclave

1. The Nonclave creates a client and makes an HTTP request to the Reverse Proxy
for the Enclave's attested certificate. While this request is not protected by
TLS, the attestation is used to prove the authenticity and integrity of the
certificate.

<!-- pluck("function", "main", "hello-https/nonclave/main.go", 47, 74) -->
```go
func main() {
	// ...
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
	// ...
}
```

2. Notice how in our Proxy we instantiate a Reverse Proxy that listens on 8080 and
forwards HTTP requests to the Enclave, as well as a Reverse TLS Proxy that listens on
8443 and forwards HTTPS requests to the Enclave.

<!-- pluck("function", "main", "hello-https/proxy/main.go", 17, 47) -->
```go
func main() {
	// ...
	revCtx, revCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer revCancel()

	revProxy, err := tee.NewReverseProxy(
		revCtx,
		config.Platform,
		config.Proxy.RevAddr,
		config.Enclave.Addr,
		logger,
	)
	if err != nil {
		logger.Error("making revProxy server", slog.String("error", err.Error()))
		return
	}
	defer revProxy.Close()

	revTLSCtx, revTLSCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer revTLSCancel()

	revProxyTLS, err := tee.NewReverseProxyTLS(
		revTLSCtx,
		config.Platform,
		config.Proxy.RevAddrTLS,
		config.Enclave.AddrTLS,
		logger,
	)
	if err != nil {
		logger.Error("making revProxyTLS server", slog.String("error", err.Error()))
		return
	}
	// ...
}
```

3. Likewise, our Enclave will need an HTTP server with a handler that returns
the Enclave's attested certificate, and an HTTPS server with a handler that
makes the requested HTTPS call on behalf of the Nonclave. For now, let's just
look at the HTTP server initialization.

<!-- pluck("function", "main", "hello-https/enclave/main.go", 24, 49) -->
```go
func main() {
	// ...
	certProvider, err := tee.NewSelfSignedCertProvider(Domain, Validity)
	if err != nil {
		logger.Error("making certProvider", slog.String("error", err.Error()))
		return
	}

	serverMux := http.NewServeMux()
	serverMux.HandleFunc(
		networking.AttestCertPath,
		networking.MakeAttestCertHandler(attester, certProvider, logger),
	)

	serverCtx, serverCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer serverCancel()
	server, err := tee.NewServer(
		serverCtx,
		config.Platform,
		config.Enclave.Addr,
		serverMux,
		logger,
	)
	if err != nil {
		logger.Error("creating server", slog.String("error", err.Error()))
		return
	}
	// ...
}
```

<!-- pluck("function", "main", "hello-https/enclave/main.go", 79, 86) -->
```go
func main() {
	// ...
	go func() {
		logger.Info("enclave server started", slog.String("addr", server.Addr()))
		err := server.Serve()
		if err != nil {
			logger.Error("enclave server error", slog.String("error", err.Error()))
		}
	}()
	// ...
}
```

4. After retrieving and verifying the attested certificate, the Nonclave creates
a client that uses the attested certificate to secure future HTTPS requests.

<!-- pluck("function", "main", "hello-https/nonclave/main.go", 75, 82) -->
```go
func main() {
	// ...
	proxyTLSURL := "https://" + net.JoinHostPort(host, strconv.Itoa(portTLS))
	clientTLS := networking.NewClient(proxyTLSURL)
	err = clientTLS.AddCertChain(verifiedCert.UserData, selfSigned)
	if err != nil {
		logger.Error("adding cert", slog.String("error", err.Error()))
		return
	}
	// ...
}
```

Here is the implementation of `AddCertChain` that adds the attested certificate
to the client's TLS configuration.

<!-- pluck("function", "Client.AddCertChain", "internal/networking/client.go", 9,36) -->
```go
func (c *Client) AddCertChain(certChainJSON []byte, selfSigned bool) error {
	// ...
	transport, ok := c.client.Transport.(*http.Transport)
	if !ok {
		return clientError("transport is not an HTTP Transport", nil)
	}

	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if transport.TLSClientConfig.RootCAs == nil {
		transport.TLSClientConfig.RootCAs = x509.NewCertPool()
	}

	// Disable hostname verification because our self-signed certs may not
	// match the name of the instance they are deployed on. That said, we still
	// perform signature verification by adding the certs to our RootCAs pool.
	if selfSigned {
		transport.TLSClientConfig.ServerName = ""
	}

	for i, certBytes := range chainDER {
		x509Cert, err := x509.ParseCertificate(certBytes)
		if err != nil {
			return fmt.Errorf("parsing chain %d: %w", i, err)
		}
		transport.TLSClientConfig.RootCAs.AddCert(x509Cert)
	}
	return nil
}
```

5. The Nonclave then asks the Enclave to make an HTTPS call on its behalf.
Remember that the Nonclave is actually hitting the Reverse TLS Proxy first, but
the TLS connection is terminated at the Enclave. The Proxy transparently
forwards the request and cannot determine what is inside.

<!-- pluck("function", "main", "hello-https/nonclave/main.go", 83, 91) -->
```go
func main() {
	// ...
	logger.Info("attesting https call", slog.String("revProxyTLS", proxyTLSURL))
	httpsCtx, httpsCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer httpsCancel()
	attestedCall, err := clientTLS.AttestHTTPSCall(httpsCtx, TargetMethod, TargetURL)
	if err != nil {
		logger.Error("attesting https call", slog.String("error", err.Error()))
		return
	}
	// ...
}
```

6. Let's take a look at how the Enclave sets up its HTTPS server. The enclave
creates a "proxied" client, which is a `http.Client` configured to send requests
to our TLS Proxy (via sockets or virtual sockets depending on the platform).

<!-- pluck("function", "main", "hello-https/enclave/main.go", 51, 77) -->
```go
func main() {
	// ...
	proxiedClient, err := tee.NewProxiedClient(config.Platform, config.Proxy.AddrTLS)
	if err != nil {
		logger.Error("making proxied client", slog.String("error", err.Error()))
		return
	}

	serverTLSMux := http.NewServeMux()
	serverTLSMux.HandleFunc(
		networking.AttestHTTPSCallPath,
		networking.MakeAttestHTTPSCallHandler(DefaultTimeout, attester, proxiedClient, logger),
	)

	serverTLSCtx, serverTLSCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer serverTLSCancel()
	serverTLS, err := tee.NewServerTLS(
		serverTLSCtx,
		config.Platform,
		config.Enclave.AddrTLS,
		serverTLSMux,
		certProvider,
		logger,
	)
	if err != nil {
		logger.Error("creating serverTLS", slog.String("error", err.Error()))
		return
	}
	// ...
}
```

7. Looking at `MakeAttestHTTPSCallHandler` we can see that the Enclave makes
the Nonclave's requested call and attests to the response.
<!-- pluck("function", "MakeAttestHTTPSCallHandler", "internal/networking/handlers.go", 12, 47) -->
```go
func MakeAttestHTTPSCallHandler(
	ctxTimeout time.Duration,
	attester *tee.Attester,
	client *http.Client,
	logger *slog.Logger,
) http.HandlerFunc {
	// ...
		req, err := http.NewRequestWithContext(ctx, httpsCallReq.Method, httpsCallReq.URL, nil)
		if err != nil {
			WriteError(w, fmt.Errorf("creating request: %w", err))
			return
		}

		logger.Info(
			"making HTTPS call",
			slog.String("method", httpsCallReq.Method),
			slog.String("URL", httpsCallReq.URL),
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

		logger.Info("attesting HTTPS call")
		attestation, err := attester.Attest(tee.WithAttestUserData(respBytes))
		if err != nil {
			WriteError(w, fmt.Errorf("attesting: %w", err))
			return
		}

		httpsCallResp := AttestHTTPSCallResponse{
			Attestation: attestation,
		}
		WriteResponse(w, httpsCallResp)
	// ...
}
```

8. Here is where our Proxy creates a TLS Proxy that the Enclave can use to make
outgoing HTTPS calls on behalf of the Nonclave. It's probably getting hard to
keep track of all the different servers and proxies at this point. Remember that
we use Proxy to refer to the application as a whole, which in this particular
example includes a reverse proxy, reverse TLS proxy, and a TLS proxy.

<!-- pluck("function", "main", "hello-https/proxy/main.go", 52, 63) -->
```go
func main() {
	// ...
	proxyTLS, err := tee.NewProxyTLS(
		proxyTLSCtx,
		config.Platform,
		config.Proxy.AddrTLS,
		logger,
	)
	if err != nil {
		logger.Error("making proxyTLS server", slog.String("error", err.Error()))
		return
	}
	defer proxyTLS.Close()
	// ...
}
```

9. When our Nonclave receives the Enclave's attested response, it verifies it
and extracts the response body. That's it! We now have an attested response from
HTTP Bin that anybody can independently verify. Moreover, we made these requests
with HTTPS, so we can now include sensitive information in our requests if needed.

<!-- pluck("function", "main", "hello-https/nonclave/main.go", 92, 114) -->
```go
func main() {
	// ...
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
```

## Next Steps

You know now how to write secure HTTPS servers and clients for cloud-based TEE
platforms! Next, try out [Hello, Expr](../hello-expr/README.md) or
[Hello, CEL](../hello-cel/README.md) to learn how to make an off-chain compute solution with TEEs.
