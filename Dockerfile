FROM golang:1.16-stretch as build

ENV GO111MODULE=on
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

# config
WORKDIR /go/src/nano-gpu-scheduler
COPY . .
# RUN GO111MODULE=on go mod download
RUN export CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' && \
    go build -ldflags="-s -w" -o /go/bin/nano-gpu-scheduler cmd/main.go

# runtime image
FROM debian:bullseye-slim

COPY --from=build /go/bin/nano-gpu-scheduler /usr/bin/nano-gpu-scheduler
