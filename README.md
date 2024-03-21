# vngcloud-csi-volume-modifier
<hr>

## Configuration
- This setup is in the **Ubuntu-22.04** environment.
  ```bash
  sudo apt install protobuf-compiler golang-goprotobuf-dev -y
  ```
  
## Generate the `*.proto` file
- Run the following command to generate the `*.pb.go` file
  ```bash
  make proto
  ```