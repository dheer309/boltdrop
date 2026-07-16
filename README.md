# Boltdrop

A cross-platform peer-to-peer file transfer CLI built in Go. Transfer files over your local network with no internet, no cloud, and no accounts required.

```
bolt send video.mp4
bolt receive
```

## How it works

BoltDrop uses mDNS to automatically discover peers on the same network. Once connected, it transfers files over a raw TCP connection using a custom chunk-manifest protocol with per-chunk SHA-256 verification and resumable transfers.

### Protocol

```
sender → receiver: manifest (JSON, length-prefixed)
receiver → sender: bitmask of completed chunks
sender → receiver: missing chunks only (index + size + data per chunk)
```

The manifest describes every chunk: its index, byte offset, size, and SHA-256 hash. The receiver verifies each chunk as it arrives and writes it directly to the correct offset in the output file using `WriteAt`, so chunks can arrive and be written in any order.

### Resumability

After each verified chunk write, the receiver persists state to a `.filename.resume` file on disk. On reconnect, the receiver reads this file and sends a bitmask back to the sender, one bit per chunk. The sender reads the bitmask and skips chunks the receiver already has, retransmitting only what is missing.

If the transfer completes successfully, the resume file is deleted automatically.

### mDNS discovery

The receiver advertises itself on the local network via `_boltdrop._tcp` using Zeroconf. The sender browses for available receivers and shows them as a numbered list to pick from.

## Project structure

```
boltdrop/
  cmd/
    bolt/
      main.go       — CLI entry point (Cobra)
      sender.go     — bolt send logic
      reciever.go   — bolt receive logic
  internal/
    transfer/
      chunk.go      — SendChunk, ReceiveChunk
      manifest.go   — SendManifest, ReadManifest, ReadCompletedChunks
      resume.go     — ResumeState, LoadResumeState, SaveResumeState
    utils/
      utils.go      — shared helpers
  chunker/
    chunker.go      — GenerateManifest, chunk hashing
```

## Installation

```bash
go install github.com/dheer309/boltdrop/cmd/bolt@latest
```

This installs the `bolt` binary to your `$GOPATH/bin`. Make sure that's in your PATH:

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

Add that line to your `~/.zshrc` or `~/.bashrc` to make it permanent.

Windows:

```
GOPROXY=direct go install github.com/dheer309/boltdrop/cmd/bolt@latest
```

Then `bolt.exe` will be available in your Go bin directory.

## Usage

Send a file:

```bash
bolt send file.mp4
```

Receive a file:

```bash
bolt receive
```

## Requirements

- Go 1.22+
- Both devices on the same local network (WiFi or Ethernet)
- No internet required
