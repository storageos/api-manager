# Build the manager binary
FROM golang:1.15.2 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY internal/ internal/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

# Use ubi8-minimal as the base image to package the manager binary. Refer to
# https://catalog.redhat.com/software/containers/ubi8/ubi-minimal/5c359a62bed8bd75a2c3fba8
# for more details
FROM registry.access.redhat.com/ubi8/ubi-minimal
WORKDIR /
COPY --from=builder /workspace/manager .

ENTRYPOINT ["/manager"]