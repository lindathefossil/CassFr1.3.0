# Contributing

Contributions to CassFr are welcome, and so are forks.

## Bugs and suggestions

Open an issue with what you did, what you expected, and what happened. If it is a reupload problem, include the text from the executable window and the chunk it happened in.

## Code

- Go code lives in `cmd/` and `internal/`, the Roblox plugin in `plugin/`.
- Run `gofmt` and `go test ./...` before opening a pull request.
- The chunking protocol is documented at the top of `internal/app/progress/progress.go` and in `cmd/assetreuploader/router_test.go`. The plugin sends chunks 1..N with `chunkIndex`, `chunkCount`, `chunkSizes` and `totalIds`, and waits for the executable to answer `done` before sending the next one.
- Everything is licensed GPL-3.0, so contributions are too.
