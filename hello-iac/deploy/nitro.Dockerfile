FROM alpine:latest
WORKDIR /app

ARG CONFIG_FILE=configs/enclave/nitro.yaml
COPY ./${CONFIG_FILE} ./config.yaml
COPY ./enclave/bin/enclave .

CMD ["/app/enclave", "--config", "/app/config.yaml"]
