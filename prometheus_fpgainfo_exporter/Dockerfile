# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

FROM golang:1.14 as builder

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download

COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o fpgainfo_exporter main.go

ARG OPAE_VERSION=$OPAE_VERSION
FROM opae:$OPAE_VERSION

USER 1001
WORKDIR /
COPY --from=builder /workspace/fpgainfo_exporter .

ENTRYPOINT ["/fpgainfo_exporter"]
