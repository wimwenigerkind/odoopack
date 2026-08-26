FROM alpine:3.24

RUN apk add --no-cache git ca-certificates

ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/odoopack /usr/local/bin/odoopack

ENTRYPOINT ["odoopack"]
