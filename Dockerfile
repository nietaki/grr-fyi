# FROM golang:1.25.6-alpine AS base
FROM golang:1.25-trixie AS base

RUN mkdir -p /usr/local/app/build
# RUN apt-get update && apt-get install -y --no-install-recommends bash
WORKDIR /usr/local/app

RUN apt-get update && apt-get install -y --no-install-recommends bash

RUN chmod ugo+rwx /usr/local/app

FROM base AS builder

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./
RUN go build -o build/grr-fyi


FROM base AS final

COPY --from=builder /usr/local/app/build/grr-fyi /usr/local/app/grr-fyi
COPY not_found.png ./
COPY *.sh ./
COPY templates ./templates
COPY views ./views
COPY static ./static


VOLUME /app_tmp

ENV SERVER_PORT=80
ENTRYPOINT ["/usr/local/app/grr-fyi"]
