#!/bin/bash
# Run our enclave and proxy programs in the background. Capture their
# STDOUT and STDERR output and prepend each line with the program name for
# better readability as their outputs are intermingled.
/app/enclave --config /app/config.yaml 2>&1 | awk '{ print "[enclave] " $0; fflush(); }' &
ENCLAVE_PID=$!

/app/proxy --config /app/config.yaml 2>&1 | awk '{ print "[proxy] " $0; fflush(); }' &
PROXY_PID=$!

# Wait for either process to exit
trap 'kill $ENCLAVE_PID $PROXY_PID; exit' TERM INT
wait -n $ENCLAVE_PID $PROXY_PID
exit 1
