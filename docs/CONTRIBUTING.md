# Contributing to app-publisher

This guide is for developers who want to build app-publisher from its source code.

## Requirements

Install the Go version required by the `go.mod` file.

## Build or install from a local checkout

Clone the repository and enter its directory:

```sh
git clone https://github.com/fgp555/app-publisher.git
cd app-publisher
```

Build the executable in the repository directory:

```sh
go build .
```

To install the executable built from this checkout into your Go binary directory, run:

```sh
go install .
```

On Windows, Go usually installs it to `%USERPROFILE%\go\bin`. Add that directory to `PATH` if you want to run `app-publisher` from any terminal.

For regular use, install the published CLI with the command in the main [README](../README.md).
