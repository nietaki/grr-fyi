FROM golang:1.25.6-alpine AS base

RUN mkdir -p /usr/local/app/build
# RUN apt-get update && apt-get install -y --no-install-recommends bash 
WORKDIR /usr/local/app


FROM base AS builder

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./
RUN go build -o build/epstein-file-review


FROM base AS final

COPY --from=builder /usr/local/app/build/epstein-file-review /usr/local/app/epstein-file-review
VOLUME /data
ENTRYPOINT ["/usr/local/app/epstein-file-review"]
