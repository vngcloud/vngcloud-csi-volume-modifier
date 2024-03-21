# vngcloud-csi-volume-modifier
<hr>

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