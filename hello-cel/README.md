# Hello, CEL

[Common Expression Language (CEL)](https://github.com/google/cel-go) is a
non-Turing complete language designed for simplicity, speed, safety, and
portability. This example demonstrates how to build a Bearclave server that
runs client-provided CEL expressions in a secure TEE environment.
Try it out yourself!

```bash
make

# You should see output similar to:
[proxy  ] time=2026-01-18T09:41:23.624-05:00 level=INFO msg="loaded config" configs/enclave/notee.yaml="&{Platform:notee Enclave:{Addr:http://127.0.0.1:8083 AddrTLS: Args:map[]} Nonclave:{Measurement: Args:map[]} Proxy:{Addr:http://127.0.0.1:8082 AddrTLS: RevAddr:http://0.0.0.0:8080 RevAddrTLS:}}"
[proxy  ] time=2026-01-18T09:41:23.624-05:00 level=INFO msg="proxy outbound server started"
[proxy  ] time=2026-01-18T09:41:23.624-05:00 level=INFO msg="proxy inbound server started"
[enclave        ] time=2026-01-18T09:41:23.722-05:00 level=INFO msg="loaded config" configs/enclave/notee.yaml="&{Platform:notee Enclave:{Addr:http://127.0.0.1:8083 AddrTLS: Args:map[]} Nonclave:{Measurement: Args:map[]} Proxy:{Addr:http://127.0.0.1:8082 AddrTLS: RevAddr:http://0.0.0.0:8080 RevAddrTLS:}}"
[enclave        ] time=2026-01-18T09:41:23.723-05:00 level=INFO msg="enclave server started" addr=127.0.0.1:8083
[nonclave       ] time=2026-01-18T09:41:23.816-05:00 level=INFO msg="loaded config" configs/nonclave/notee.yaml="&{Platform:notee Enclave:{Addr: AddrTLS: Args:map[]} Nonclave:{Measurement:Not a TEE platform. Code measurements are not real. Args:map[]} Proxy:{Addr: AddrTLS: RevAddr: RevAddrTLS:}}"
[enclave        ] time=2026-01-18T09:41:23.819-05:00 level=INFO msg="received attest CEL request"
[enclave        ] time=2026-01-18T09:41:23.819-05:00 level=INFO msg="executing cel" expression="httpGet(targetUrl).url == targetUrl ? \"URL Match Success\" : \"URL Mismatch\""
[proxy  ] time=2026-01-18T09:41:23.822-05:00 level=INFO msg="forwarding request" url=http://httpbin.org/get
[enclave        ] time=2026-01-18T09:41:23.928-05:00 level=INFO msg="attesting cel" result="{Expression:httpGet(targetUrl).url == targetUrl ? \"URL Match Success\" : \"URL Mismatch\" Env:map[targetUrl:http://httpbin.org/get] Output:URL Match Success}"
[nonclave       ] time=2026-01-18T09:41:23.929-05:00 level=INFO msg="verified attestation"
[nonclave       ] time=2026-01-18T09:41:23.929-05:00 level=INFO msg="attested cel" expression="httpGet(targetUrl).url == targetUrl ? \"URL Match Success\" : \"URL Mismatch\"" env=map[targetUrl:http://httpbin.org/get]
[nonclave       ] time=2026-01-18T09:41:23.929-05:00 level=INFO msg="expression result:" value="URL Match Success"
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
