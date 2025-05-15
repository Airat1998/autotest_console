FROM golang:1.23-alpine AS build_hk_panel_test

WORKDIR /build

COPY . .

RUN go mod init hk_panel_test && \
    go mod tidy && \
    go build -o hk_panel_test

FROM alpine:latest AS hk_panel_test

WORKDIR /app

COPY --from=build_hk_panel_test /build/hk_panel_test .
