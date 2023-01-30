## SPDX-License-Identifier: Apache-2.0
## Copyright (c) 2020-2023 Intel Corporation

# Build the manager binary
FROM golang:1.18.3 as builder

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
COPY pkg/ pkg/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

FROM registry.access.redhat.com/ubi9/ubi-micro:9.1.0-13

ARG VERSION
### Required OpenShift Labels
LABEL name="SR-IOV Operator for Wireless FEC Accelerators" \
    vendor="Intel Corporation" \
    version=$VERSION \
    release="1" \
    summary="SR-IOV Operator for Wireless FEC Accelerators for 5G Cloudnative/vRAN deployment" \
    description="SR-IOV Operator for Wireless FEC Accelerators ACC100 and ACC200 for 5G Cloudnative/vRAN deployment"

COPY TEMP_LICENSE_COPY /licenses/LICENSE

WORKDIR /
COPY --from=builder /workspace/manager .
COPY assets assets/

USER 1001

ENTRYPOINT ["/manager"]
