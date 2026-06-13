# Specs

A file server with primary purpose of serving and downloading files in the local network with ease.

## V1

- [x] Create a basic file server that can serve files over HTTP.
- [x] Show a QR code in the terminal that points to the file server URL in the local network.

## V1.1

- [x] Show logs and download progress in the terminal.
- [x] Support resume downloads.


## Next versions
- [x] Show the progress and QR code with tui
- [x] Support basic dowloading directory as archive
- [x] Archive without loading all files into memory
- [x] Support resumable archive
- [ ] Support If-Range header
- [ ] Support HEAD requests
- [ ] Support non resumable compressed archive
- [ ] Fix tui for unknown file sizes
- [ ] Handle symlinks when archiving files
- [ ] Support static sites via `index.html`
- [ ] Support SPA via flag
- [ ] Configrable log/log file
- [ ] Support tar
- [ ] Support basic auth
- [ ] Support encrypted archives
- [ ] Allow upload via flag

## Ideas

- [ ] Show more info on tui, with more styling
- [ ] Generate and show device name based on user agent.
- [ ] Explore [zeroconf](https://github.com/grandcat/zeroconf) and see how it can be useful in this project
