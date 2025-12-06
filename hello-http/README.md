# Hello HTTP
Bearclave adapts to different TEE platforms by requiring two programs:
- **Enclave**: Runs securely within the TEE and now functions as a standard
  HTTP server.
- **Proxy**: Acts as an HTTP reverse proxy, bridging communication between
  the client and the enclave. It forwards requests using traditional sockets or
  VSOCK, depending on the platform.

For AWS Nitro Enclaves, which rely on VSOCK interfaces, the proxy translates
client HTTP requests into VSOCK communication and routes them to the enclave.
On AMD SEV-SNP and Intel TDX, where the networking stack is directly accessible
within the TEE, the proxy still runs but operates over standard networking.

### Workflow Example
This example showcases an HTTP-based attestation flow:
1. A **remote client** ("nonclave") sends an HTTP request to the proxy with
   user data for attestation.
2. The **proxy** forwards the request to the enclave via a traditional socket
   or VSOCK, depending on the platform.
3. The **enclave**, now an HTTP server, processes the HTTP request, generates
   an attestation report, and sends it back as an HTTP response.
4. The **proxy** relays the response back to the remote client.
5. The **nonclave** verifies the attestation report and extracts the attested
   data.

### Key Points
- The enclave operates as a complete HTTP server, allowing developers to
  use standard HTTP-based applications without significant changes to leverage
  TEE environments.
- The proxy transparently handles platform-specific differences, simplifying
  deployment across AWS Nitro, AMD SEV-SNP, and Intel TDX.


## Run the example locally
Bearclave supports a "No TEE" mode (`notee`) for running code locally. This
allows for quick iteration and saves on Cloud infrastructure costs. Assuming
you have installed the minimum set of dependencies listed in the
[top-level README](../../README.md), you can run this example locally with:

```bash
make

# You should see output similar to:
[enclave-proxy  ] time=2025-07-20T11:41:34.789-04:00 level=INFO msg="loaded config" configs/enclave/notee.yaml="&{Platform:notee Measurement: IPCs:map[] Servers:map[enclave-server:{CID:4 Port:8082 Route:}] Proxy:{Port:8080}}"
[enclave-proxy  ] time=2025-07-20T11:41:34.789-04:00 level=INFO msg="Proxy server started" addr=0.0.0.0:8080
[enclave        ] time=2025-07-20T11:41:34.793-04:00 level=INFO msg="loaded config" configs/enclave/notee.yaml="&{Platform:notee Measurement: IPCs:map[] Servers:map[enclave-server:{CID:4 Port:8082 Route:}] Proxy:{Port:8080}}"
[enclave        ] time=2025-07-20T11:41:34.793-04:00 level=INFO msg="Enclave server started" addr=127.0.0.1:8082
[nonclave       ] time=2025-07-20T11:41:35.131-04:00 level=INFO msg="loaded config" configs/nonclave/notee.yaml="&{Platform:notee Measurement:Not a TEE platform. Code measurements are not real. IPCs:map[] Servers:map[] Proxy:{Port:8080}}"
[enclave        ] time=2025-07-20T11:41:35.132-04:00 level=INFO msg="attesting userdata" userdata="Hello, world!"
[nonclave       ] time=2025-07-20T11:41:35.133-04:00 level=INFO msg="verified userdata" userdata="Hello, world!"
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
   cd bearclave/examples/hello-http
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
