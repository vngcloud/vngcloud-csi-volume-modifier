# vngcloud-csi-volume-modifier
<hr>

**DEV** - [![Package vngcloud-csi-volume-modifier into container image on DEV branch](https://github.com/vngcloud/vngcloud-csi-volume-modifier/actions/workflows/build_dev.yml/badge.svg)](https://github.com/vngcloud/vngcloud-csi-volume-modifier/actions/workflows/build_dev.yml)

**PROD** - [![Package vngcloud-csi-volume-modifier into container image on MAIN branch](https://github.com/vngcloud/vngcloud-csi-volume-modifier/actions/workflows/build_prod.yml/badge.svg)](https://github.com/vngcloud/vngcloud-csi-volume-modifier/actions/workflows/build_prod.yml)

**RELEASE** - [![Release vngcloud-csi-volume-modifier project](https://github.com/vngcloud/vngcloud-csi-volume-modifier/actions/workflows/release.yml/badge.svg)](https://github.com/vngcloud/vngcloud-csi-volume-modifier/actions/workflows/release.yml)

## Configuration
- This setup is in the **Ubuntu-22.04** environment.
  ```bash
  sudo apt install protobuf-compiler -y && \
  go install google.golang.org/protobuf/cmd/protoc-gen-go && \
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc && \
  export PATH="$PATH:$(go env GOPATH)/bin"
  ```
  
## Generate the `*.proto` file
- Run the following command to generate the `*.pb.go` file _(Make sure complete section [Configuration](#configuration) before run this command)_
  ```bash
  make proto
  ```
  
## Build the Docker image
- Run the following command to build the Docker image
  ```bash
  make docker-build && make docker-push
  ```