FROM --platform=$BUILDPLATFORM golang:1.22.0 AS builder
WORKDIR /go/src/github.com/vngcloud/vngcloud-csi-volume-modifier
COPY go.* .
ARG GOPROXY=direct
RUN go mod download
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION="v0.0.0"
RUN OS=$TARGETOS ARCH=$TARGETARCH make $TARGETOS/$TARGETARCH

FROM registry.k8s.io/build-image/go-runner:v2.3.1-go1.22.0-bookworm.0 AS linux-vngcloud
COPY --from=builder /go/src/github.com/vngcloud/vngcloud-csi-volume-modifier/bin/vngcloud-csi-volume-modifier /bin/vngcloud-csi-volume-modifier
ENTRYPOINT ["/bin/vngcloud-csi-volume-modifier"]
