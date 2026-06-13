# le

`le` is a simple local-network file server written in Go. Start it in a directory, scan the QR code, and download files from another device on the same network.

## Features

- Serve any local directory over HTTP.
- Show the local network address and QR code in a terminal TUI.
- Download directories as ZIP archives, including a resumable uncompressed ZIP option. ZIP downloads are streamed directly, so `le` __does not__ create a temporary archive on disk or load the whole archive into memory.
- Track active downloads and transfer progress in the terminal.
- Resume interrupted file downloads with HTTP range requests.
- Browse folders in a clean, responsive browser UI.
- Keep command-line access simple for tools like `curl` and `wget`.

## Install

Install the `le` binary permanently with:

```sh
go install go.sakib.dev/le@latest
```

Make sure your Go binary directory is on `PATH`. It is usually:

```sh
$(go env GOPATH)/bin
```

After that, run `le` from any directory:

```sh
le
```

## Quick start

Run `le` for the current directory:

```sh
go run go.sakib.dev/le@latest
```

Open the URL shown in the terminal, or scan the QR code from another device on the same network.


## Options

- `--dir`: Directory to serve files from. Defaults to the current directory.
- `--port`: Port to run the server on. Defaults to `8080`.
