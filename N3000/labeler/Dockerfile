# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

FROM golang:1.14 as builder

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download

COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o node_labeler main.go

FROM registry.access.redhat.com/ubi8:latest

RUN mkdir -p /usr/share/misc/ && \
    cd /usr/share/misc/ && \
    curl https://pci-ids.ucw.cz/v2.2/pci.ids.gz --output pci.ids.gz && \
    gzip -d pci.ids.gz && \
    rm -rfv ./pci.ids.gz

WORKDIR /labeler-workspace
COPY --from=builder /workspace/node_labeler .

ENTRYPOINT ["bash", "-c", "/labeler-workspace/node_labeler && sleep infinity"]
