# Hello World
Bearclave supports multiple TEE platforms, which influences application design.
Specifically, two programs are required:
- **Enclave**: The code running securely within the TEE.
- **Proxy**: A bridging program for communication between the enclave and
  external clients.

For AWS Nitro Enclaves, this architecture is necessary because enclaves use
VSOCK (virtual socket) interfaces, requiring a proxy to handle traditional
socket-based communications (e.g., HTTP) with external clients. This proxy
forwards data through a VSOCK to the enclave. In contrast, AMD SEV-SNP and
Intel TDX allow enclaves direct access to the networking stack, so the proxy
can run inside the TEE, making it less essential but still included for
consistency.

### Workflow Example
This example demonstrates a simple attestation flow:
1. A **remote client** ("nonclave") sends an HTTP request for enclave
   attestation over some data.
2. The **proxy** receives the HTTP request, unwraps the data, and forwards it
   via a VSOCK to the enclave.
3. The **enclave** processes the data, generates an attestation, and sends the
   report back to the proxy via the VSOCK.
4. The **proxy** wraps the attestation into an HTTP response and returns it to
   the client.
5. The **nonclave** verifies the attestation and extracts the attested data.

Key points:
- The enclave communicates using a minimalist protocol: listening for a byte
  stream via VSOCK and assuming it's user data.
- For simplicity, the enclave could be designed as a standard HTTP server
  (demonstrated in the [hello-http](../hello-http/README.md) example),
  avoiding the need for developers to rewrite applications for the TEE.

## Run the example locally
Bearclave supports a "No TEE" mode (`notee`) for running code locally. This
allows for quick iteration and saves on Cloud infrastructure costs. Assuming
you have installed the minimum set of dependencies listed in the
[top-level README](../../README.md), you can run this example locally with:

```bash
make

# You should see output similar to:
[enclave        ] time=2025-07-20T10:25:04.534-04:00 level=INFO msg="loaded config" configs/enclave/notee.yaml="&{Platform:notee Measurement: IPCs:map[enclave:{Endpoint:127.0.0.1:8083} enclave-proxy:{Endpoint:127.0.0.1:8082}] Servers:map[] Proxy:{Port:8080}}"
[enclave        ] time=2025-07-20T10:25:04.534-04:00 level=INFO msg="waiting to receive userdata from enclave-proxy..."
[enclave-proxy  ] time=2025-07-20T10:25:04.564-04:00 level=INFO msg="loaded config" configs/enclave/notee.yaml="&{Platform:notee Measurement: IPCs:map[enclave:{Endpoint:127.0.0.1:8083} enclave-proxy:{Endpoint:127.0.0.1:8082}] Servers:map[] Proxy:{Port:8080}}"
[enclave-proxy  ] time=2025-07-20T10:25:04.564-04:00 level=INFO msg="HTTP server started" addr=0.0.0.0:8080
[nonclave       ] time=2025-07-20T10:25:04.904-04:00 level=INFO msg="loaded config" configs/nonclave/notee.yaml="&{Platform:notee Measurement:Not a TEE platform. Code measurements are not real. IPCs:map[] Servers:map[] Proxy:{Port:8080}}"
[enclave-proxy  ] time=2025-07-20T10:25:04.904-04:00 level=INFO msg="sending userdata to enclave..." userdata="Hello, world!"
[enclave-proxy  ] time=2025-07-20T10:25:04.904-04:00 level=INFO msg="waiting for attestation from enclave..."
[enclave        ] time=2025-07-20T10:25:04.904-04:00 level=INFO msg="attesting userdata" userdata="Hello, world!"
[enclave        ] time=2025-07-20T10:25:04.904-04:00 level=INFO msg="sending attestation to enclave-proxy..."
[enclave        ] time=2025-07-20T10:25:04.905-04:00 level=INFO msg="waiting to receive userdata from enclave-proxy..."
[nonclave       ] time=2025-07-20T10:25:04.905-04:00 level=INFO msg="verified userdata" userdata="Hello, world!"
```

## Run the example on AWS Nitro
Recall that AWS Nitro Enclave development requires building and deploying
from the EC2 instance itself. Assuming you have configured your AWS account
and the necessary cloud resources laid out in the
[AWS setup guide](../../docs/AWS.md), you can run the example with:

1. Login to the AWS cli
    ```bash
    make aws-cli-login 
    ```
2. Start your EC2 instance
    ```bash
    make aws-nitro-instance-start
   ```
3. Log into your EC2 instance
    ```bash
   make aws-nitro-instance-ssh 
   ```
4. Clone (if needed) bearclave and cd to example
    ```bash
   git clone git@github.com:tahardi/bearclave.git
   cd bearclave/examples/hello-world
   ```
5. Build and run the enclave program
    ```bash
    make aws-nitro-enclave-run-eif
    
    # This will attach a debug console to the enclave program so you can see
    # its log. Note that this will change the code measurement, however, to
    # indicate the enclave is in debug mode (and thus may leak sensitive info)
    make aws-nitro-enclave-run-eif-debug
    ```
6. Run the proxy program
    ```bash
   make aws-nitro-proxy-run 
   ```
7. In a separate terminal window, run the nonclave program. Remember this is a
   remote client making an HTTP request so you can run it from your local machine
    ```bash
    make aws-nitro-nonclave-run
    ```
8. Remember to turn off your EC2 instance when you are finished. Otherwise, you
   will continue to incur AWS cloud charges.
    ```bash
    make aws-nitro-instance-stop 
    ```

## Run the example on GCP AMD SEV-SNP or Intel TDX
Unlike AWS Nitro, the GCP AMD and Intel programs are built and deployed from
your local machine. Assuming you have configured your GCP account and the
necessary cloud resources laid out in the [GCP setup guide](../../docs/GCP.md),
you can run the example with:

1. Start your Compute instance. Note that these steps demonstrate running the
   example on AMD SEV-SNP, but you can simply replace `sev` with `tdx` in the make
   commands as the process is exactly the same.
    ```bash
    make gcp-sev-instance-start
   ```
2. Build and deploy the enclave and proxy programs. Note that unlike AWS Nitro,
   the enclave and proxy are bundled together and both deployed within the TEE
   when run on SEV or TDX.
    ```bash
    make gcp-sev-enclave-run-image
    ```
3. Run the nonclave program
    ```bash
   make gcp-sev-nonclave-run
   ```
4. Remember to turn off your Compute instance when you are finished. Otherwise,
   you will continue to incur GCP cloud charges.
    ```bash
    make gcp-sev-instance-stop 
    ```
