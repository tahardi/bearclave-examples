# Hello, World

This example serves as an introduction to writing TEE-applications with the
[Bearclave SDK](https://github.com/tahardi/bearclave). This includes how to
write, build, and deploy applications to real cloud-based TEE platforms. For
an overview on what TEEs are and the different platforms (e.g., AWS Nitro
Enclaves, AMD SEV-SNP, Intel TDX) refer to the
[TEE concepts](https://github.com/tahardi/bearclave/blob/main/docs/concepts.md)
document.

## Application Structure

Bearclave applications are split into two programs: a Proxy and an Enclave.
This architecture is necessary to run on the AWS Nitro Enclave platform. For
the sake of consistency and developer experience, we assume this architecture
for AMD SEV-SNP and Intel TDX as well. Doing so allows us to write applications
without worrying about the underlying TEE platform.

### Enclave

We use the term **Enclave** to refer to the program that executes within a TEE
and contains your business logic and data. As the program running within a TEE,
the Enclave has certain properties and functionalities not afforded to "normal"
programs. The confidentiality and integrity of the Enclave's code and data is
assured, even in the face of a malicious Host OS or Hypervisor. The Enclave can
_prove_ this to outside parties by generating attestation reports. As we will
demonstrate in this example, attestation reports can also be used to "witness"
arbitrary "user data". This is especially useful if you want to prove to some
other party that your Enclave program has seen or generated a given set of data.

### Proxy

We use the term **Proxy** to refer to the program that handles translating and
forwarding communications between the Enclave and clients. It is important to
note that **the Proxy is outside our trust boundary**. Because of the Nitro
Enclave architecture, we have to always assume that the Proxy is run outside
the TEE (even though that is not true for SEV-SNP and TDX). The only assumption
we make is that the Proxy will (eventually) forward our requests and responses.
If we are receiving or sending sensitive data, however, we need to take proper
precautions (e.g., use authenticated encryption) to ensure the Proxy cannot
modify or read our data.

### Nonclave

We use the term **Nonclave** to refer to the "non-Enclave" program, which
represents the client(s) of our application. They may want to verify that
the Enclave is running within a genuine TEE before sending over sensitive
data or asking the Enclave to perform some action. This is done by requesting
and verifying an attestation report, which we will demonstrate in this example.

## Walkthrough

This example demonstrates a simple workflow, where the Nonclave wants the
Enclave to "witness" some data. The Nonclave sends an HTTP request
containing the data to witness to our Proxy. The Proxy unpacks the data and
sends it to the Enclave via a socket (read the AWS Nitro Enclaves section
of the
[TEE Overview]([TEE concepts](https://github.com/tahardi/bearclave/blob/main/docs/concepts.md))
for details on why we cannot send requests directly to the Enclave).
The Enclave generates an attestation report containing the user data, which
then gets returned to the Proxy and ultimately to the Nonclave. Upon receiving
the response, the Nonclave verifies the attestation report. Below we will walk
through each step in more detail and examine the relevant code.

1. a
<!-- pluck("function", "main", "hello-world/nonclave/main.go", 60, 79) -->
```go

```

1. a
<!-- pluck("function", "main", "hello-world/nonclave/main.go", 60, 79) -->
```go

```

## Running Locally


## Running on the Cloud

