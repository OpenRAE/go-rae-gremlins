# Docker

go-rae-gremlins publishes no Docker image.
To run Gremlins in a container, use the official [`golang` image](https://hub.docker.com/_/golang) and install Gremlins with `go install`.

Before you start:

- Replace `<version>` with a version from the [release page](https://github.com/OpenRAE/go-rae-gremlins/releases).
- Use a `golang` image with Go 1.25 or later that can also build your module.

## As a pipeline step

In your continuous integration tool, run a step with this command:

```shell
docker run --rm -v "$(pwd)":/app -w /app golang:1.25 \
  sh -c 'go install github.com/OpenRAE/go-rae-gremlins/cmd/gremlins@v<version> && gremlins unleash .'
```

The command mounts the current directory into the container with write access.
The tests of your module and Gremlins can write to that directory.
The syntax of a pipeline step depends on your tool.

## As a stage in the Dockerfile

Add a stage to your `Dockerfile`:

```dockerfile
FROM golang:1.25 AS mutation-testing
RUN go install github.com/OpenRAE/go-rae-gremlins/cmd/gremlins@v<version>
WORKDIR /my/project/source
COPY . .
RUN gremlins unleash
```

A failed `gremlins unleash` command, for example a threshold that is not met, stops the build of the stage.
For more information, see the [Docker multi-stage builds documentation](https://docs.docker.com/build/building/multi-stage/).
