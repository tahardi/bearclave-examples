FROM alpine:latest

# Add tini for better process management
RUN apk add --no-cache tini bash

WORKDIR /app

ARG CONFIG_FILE=configs/enclave/tdx.yaml
COPY ./${CONFIG_FILE} ./config.yaml
COPY ./enclave/bin/enclave .
COPY ./proxy/bin/proxy .
COPY ./tdx-run.sh .
RUN chmod +x ./tdx-run.sh

# Use tini as the entry point
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/app/tdx-run.sh"]
