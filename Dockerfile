FROM --platform=$BUILDPLATFORM golang:1.21 AS builder
WORKDIR /go/src/github.com/vngcloud/vngcloud-csi-volume-modifier
COPY go.* .
ARG GOPROXY=direct
RUN go mod download
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION="v0.0.0"
RUN OS=$TARGETOS ARCH=$TARGETARCH make $TARGETOS/$TARGETARCH

FROM vcr.vngcloud.vn/60108-cuongdm3/go-runner:v2.3.1-go1.21.7-bullseye.0 AS linux-vngcloud
COPY --from=builder /go/src/github.com/vngcloud/vngcloud-csi-volume-modifier/bin/vngcloud-csi-volume-modifier /bin/vngcloud-csi-volume-modifier
ENTRYPOINT ["/bin/vngcloud-csi-volume-modifier"]
