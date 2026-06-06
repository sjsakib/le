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
- [ ] Archive without loading all files into memory
- [ ] Support resumable archive
- [ ] Support non resumable compressed archive
- [ ] Fix tui for unknown file sizes

## Ideas

- [ ] Show more info on tui, with more styling
- [ ] Generate and show device name based on user agent.
- [ ] Explore [zeroconf](https://github.com/grandcat/zeroconf) and see how it can be useful in this project
