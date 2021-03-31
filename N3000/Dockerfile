# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020-2021 Intel Corporation

# Build the manager binary
FROM golang:1.15 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ARG VERSION
### Required OpenShift Labels
LABEL name="OpenNESS Operator for Intel® FPGA PAC N3000" \
    vendor="Intel Corporation" \
    version=$VERSION \
    release="1" \
    summary="OpenNESS Operator for Intel® FPGA PAC N3000 for 5G Cloudnative/vRAN deployment" \
    description="OpenNESS Operator for Intel® FPGA PAC N3000 for 5G Cloudnative/vRAN \
deployment supports high performance Networking interface for 5G gNB Fronthaul and Midhaul \
and 5G Forward Error Correction offload. OpenNESS Operator for Intel® FPGA PAC N3000 supports \
Detection of the FPGA, Programming FPGA and Ethernet Firmware, Configuration and allocation \
of SR-IOV resources to the O-DU Pods."

COPY TEMP_LICENSE_COPY /licenses/LICENSE

WORKDIR /
COPY --from=builder /workspace/manager .

USER nobody

COPY assets/ assets/
ENTRYPOINT ["/manager"]
