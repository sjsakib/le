# le

`le` is a simple local-network file server written in Go. Start it in a directory, scan the QR code, and download files from another device on the same network.

The name `le` comes from Bengali slang meaning "take it."

## Features

- Serve any local directory over HTTP.
- Show the local network address and QR code in a terminal TUI.
- Download directories as ZIP archives, including a resumable uncompressed ZIP option. ZIP downloads are streamed directly, so `le` **does not** create a temporary archive on disk or load the whole archive into memory.
- Track active downloads and transfer progress in the terminal.
- Resume interrupted file downloads with HTTP range requests.
- Browse folders in a clean, responsive browser UI.
- Keep command-line access simple for tools like `curl` and `wget`.

## Quick run

Run `le` for the current directory if you have Go installed:

```sh
go run go.sakib.dev/le@latest
```

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

## Options

- `--dir`: Directory to serve files from. Defaults to the current directory.
- `--port`: Port to run the server on. Defaults to `8080`.
