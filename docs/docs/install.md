# Install

Install the `gremlins` binary from a release archive or from source.
To run Gremlins, you also need Go 1.25 or later, because Gremlins runs `go test`.

## Pre-compiled binaries

Each [release](https://github.com/OpenRAE/go-rae-gremlins/releases/latest) has archives for these systems:

| System | Architectures |
| --- | --- |
| Linux | `386`, `amd64`, `arm64` |
| macOS | `amd64`, `arm64` |
| Windows | `386`, `amd64`, `arm64` |

All archives are `.tar.gz` files. A release also has a `checksums.txt` file with the SHA-256 of each archive.

1. Download the archive for your system and the `checksums.txt` file of the same release.
    The archive name is `gremlins_<version>_<os>_<arch>.tar.gz`.

2. Check the archive. For example, on Linux:

    ```shell
    sha256sum --check --ignore-missing checksums.txt
    ```

    The command prints `OK` for the archive. If it does not, download the archive again.

3. Extract the archive:

    ```shell
    tar -xzf gremlins_<version>_linux_amd64.tar.gz
    ```

4. Copy the `gremlins` binary to a directory in your `PATH`. For example:

    ```shell
    sudo cp gremlins /usr/local/bin
    ```

## From source

### Go install

You need the [Go toolchain](https://go.dev/dl/), 1.25 or later.

```shell
go install github.com/OpenRAE/go-rae-gremlins/cmd/gremlins@v<version>
```

The `go` command puts the binary in `$(go env GOPATH)/bin`, or in `GOBIN` if it is set.

### Build

You need the [Go toolchain](https://go.dev/dl/), Git, and `make`.

```shell
git clone https://github.com/OpenRAE/go-rae-gremlins.git
cd go-rae-gremlins
make build
```

The binary is `dist/bin/gremlins`.

## Check the installation

Run this command:

```shell
gremlins --version
```

The output gives the version, the operating system, and the architecture. For example:

```text
gremlins version 0.10.0 linux/amd64
```
