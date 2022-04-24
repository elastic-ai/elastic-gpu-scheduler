FROM golang:1.16-stretch as build

ENV GO111MODULE=on
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

# config
WORKDIR /go/src/elastic-gpu-scheduler
COPY . .
# RUN GO111MODULE=on go mod download
RUN export CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' && \
    go build -ldflags="-s -w" -o /go/bin/elastic-gpu-scheduler cmd/main.go

# runtime image
FROM debian:bullseye-slim

COPY --from=build /go/bin/elastic-gpu-scheduler /usr/bin/elastic-gpu-scheduler
