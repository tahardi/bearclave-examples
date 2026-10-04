# Hello, IaC

"Attested infrastructure as code" means proving that a specific script produced
a specific infrastructure plan. This example runs client-provided
[Risor](https://github.com/deepnoodle-ai/risor) scripts in a secure TEE
environment and attests to both the script and the plan it generated. Try it
out yourself!

```bash
make

# You should see output similar to:
[enclave	] level=INFO msg="received attest IaC request"
[enclave	] level=INFO msg="executing script" script="..."
[enclave	] level=INFO msg="attesting plan" plan="[{\"name\":\"bearclave-logs-us-east-1\",\"region\":\"us-east-1\",\"type\":\"aws_s3_bucket\"},...]"
[nonclave	] level=INFO msg="verified attestation"
[nonclave	] level=INFO msg="verified plan" plan="[{\"name\":\"bearclave-logs-us-east-1\",\"region\":\"us-east-1\",\"type\":\"aws_s3_bucket\"},...]"
```

## How it Works

1. The Client writes a Risor script that returns a list of resources and sends
it to the Enclave.
2. The Enclave runs the script in a sandbox. Risor starts with an empty
environment, so scripts have no access to the filesystem, network, or OS. The
Enclave adds only Risor's pure builtins and leaves out the `rand` module, so the
same script always produces the same plan.
3. The Enclave encodes the result as JSON and attests to
`sha256(script) || sha256(plan)`. It returns the plan and the attestation.
4. The Client verifies the attestation, then hashes its own script and the
returned plan. If either hash differs from the attested ones, the plan was not
produced by that script and the Client rejects it.

## Next Steps

Try adding a whitelisted function that reads approved inputs, such as a list of
regions from a config service, and include those inputs in the attestation.
