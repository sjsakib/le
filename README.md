# le
`le` is a simple file server written in Go with
* resume support
* local address QR code
* download logs with progress
* clean browser UI for directory browsing

## Usage

```sh
go run go.sakib.dev/le
```

## Optional parameters
- `--dir`: Directory to serve files from (default: current directory)
- `--port`: Port to run the server on (default: 8080)

## Browser UI
When accessed from a web browser, `le` serves a clean, responsive interface featuring:
- File and folder icons
- Human-readable file sizes
- Relative timestamps
- Breadcrumb navigation
- Mobile-friendly design

Command-line tools like `curl` or `wget` still get the simple directory listing for easy parsing.

Browser UI Preview:

┌─────────────────────────────────────────────────────────────┐
│ 📁 Index of /test_browser                                   │
│ ─────────────────────────────────────────────────────────── │
│ Root / test_browser                                         │
│                                                             │
│ 📁 code/                              2 minutes ago         │
│ 📁 documents/                         2 minutes ago         │
│ 📁 images/                            2 minutes ago         │
│ 📄 data.json              18 B        2 minutes ago         │
│                                                             │
│                    Served by le                             │
└─────────────────────────────────────────────────────────────┘