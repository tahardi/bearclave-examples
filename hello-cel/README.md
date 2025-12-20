# Hello, CEL

[Common Expression Language (CEL)](https://github.com/google/cel-go) is a
non-Turing complete language designed for simplicity, speed, safety, and
portability. This example demonstrates how to build a Bearclave server that
runs client-provided CEL expressions in a secure TEE environment.
Try it out yourself!

```bash
make

# You should see output similar to:
[proxy  ] time=2025-12-20T14:45:38.991-05:00 level=INFO msg="loaded config" configs/enclave/notee.yaml="&{Platform:notee Enclave:{Network:tcp Addr:http://127.0.0.1:8083 Route:app/v1} Nonclave:{Measurement: Route:} Proxy:{Network:tcp InAddr:http://0.0.0.0:8080 OutAddr:http://127.0.0.1:8082}}"
[proxy  ] time=2025-12-20T14:45:38.991-05:00 level=INFO msg="proxy outbound server started"
[proxy  ] time=2025-12-20T14:45:38.991-05:00 level=INFO msg="proxy inbound server started"
[enclave        ] time=2025-12-20T14:45:39.537-05:00 level=INFO msg="loaded config" configs/enclave/notee.yaml="&{Platform:notee Enclave:{Network:tcp Addr:http://127.0.0.1:8083 Route:app/v1} Nonclave:{Measurement: Route:} Proxy:{Network:tcp InAddr:http://0.0.0.0:8080 OutAddr:http://127.0.0.1:8082}}"
[enclave        ] time=2025-12-20T14:45:39.537-05:00 level=INFO msg="enclave server started" addr=127.0.0.1:8083
[nonclave       ] time=2025-12-20T14:45:40.061-05:00 level=INFO msg="loaded config" configs/nonclave/notee.yaml="&{Platform:notee Enclave:{Network: Addr: Route:} Nonclave:{Measurement:Not a TEE platform. Code measurements are not real. Route:app/v1} Proxy:{Network: InAddr: OutAddr:}}"
[enclave        ] time=2025-12-20T14:45:40.062-05:00 level=INFO msg="executing cel" expression="httpGet(targetUrl).url == targetUrl ? \"URL Match Success\" : \"URL Mismatch\""
[proxy  ] time=2025-12-20T14:45:40.063-05:00 level=INFO msg="forwarding request" url=http://httpbin.org/get
[enclave        ] time=2025-12-20T14:45:40.240-05:00 level=INFO msg="attesting cel" hash="NTP1x0ckuRyhi9bs7EWP/a/bQ5nF85Av1FJUhCy4LIs="
[nonclave       ] time=2025-12-20T14:45:40.240-05:00 level=INFO msg="verified attestation"
[nonclave       ] time=2025-12-20T14:45:40.240-05:00 level=INFO msg="verified cel" hash="NTP1x0ckuRyhi9bs7EWP/a/bQ5nF85Av1FJUhCy4LIs="
[nonclave       ] time=2025-12-20T14:45:40.240-05:00 level=INFO msg="expression result:" value="URL Match Success"
```

## How it Works

Check out the [Hello, Expr](../hello-expr/README.md) for a detailed walkthrough
of how this example works. These examples are identical except that one runs
an Expr emulator and the other a CEL. Interestingly, the CEL and Expr languages
are so similar that the exact same expression used in this example works is
also used in the Expr example!

## Next Steps

You now know how to execute arbitrary Client CEL and Expre expressions in a
secure TEE environment. Try combining them and running a single server that
supports both!
