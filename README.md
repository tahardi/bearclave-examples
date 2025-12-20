# Bearclave Examples

This repository contains a collection of examples that demonstrate how to
build real-world TEE applications using the Bearclave SDK.

## Getting Started

We recommend that you view examples in the following order, as each example
builds upon the previous one:
- [**Hello, World**](./hello-world) an introduction to Bearclave application
development. This covers application design, building, and deployment.
- [**Hello, HTTP**](./hello-http) an introduction to networking with Bearclave
applications. This covers running HTTP clients and servers inside enclaves.
- [**Hello, Expr**](./hello-expr) an example demonstrating how to run an Expr
language runtime inside an enclave for executing and attesting to arbitrary
client-provided expressions.
- [**Hello, CEL**](./hello-cel) an example demonstrating how to run a Common
Expression Language (CEL) runtime inside an enclave for executing and attesting
to arbitrary client-provided expressions.
