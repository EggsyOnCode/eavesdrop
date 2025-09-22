# Eavesdrop

A small experimental project exploring concepts from Chainlink's Offchain Reporting (OCR) and Cross-Chain Interoperability Protocol (CCIP). This codebase is a WIP prototype intended for learning and experimentation, not production use.

- Inspired by Chainlink’s OCR and CCIP whitepapers and public documentation
- Implements a subset of ideas: committees, leader election, observations, reporting, and a simple transport/RPC layer
- Uses Go, with minimal dependencies and a local, in-process demo entrypoint


## Protocol inspiration and disclaimer
This is an incomplete, work-in-progress implementation of a protocol inspired by Chainlink’s OCR and CCIP. It does not implement the full protocols, security guarantees, or edge-case handling described in the whitepapers. Use at your own risk; do not use in production or with real funds.

- OCR inspiration: committee-based observation/aggregation, leader rotation (pacemaker), report assembly, and transmission
- CCIP inspiration: message/report transport abstractions and serialization


## Requirements
- Go 1.22.2 or newer (module `go 1.22.2`)
- Linux/macOS recommended (Makefile provided). Windows may work via WSL

Optional:
- `make` for convenient build/run/test targets


## Quick start
Build and run using the provided targets:

```bash
make build
make run
```

Run tests:

```bash
make test
```

Alternatively, run directly:

```bash
go run ./...
```

The default `main.go` launches three in-process OCR servers with randomly generated private keys and blocks indefinitely.


## Project structure
A high-level overview of the modules and their responsibilities.

- `main.go`: Example bootstrap that creates three servers, wraps each in an OCR instance, and starts them concurrently.

- `crypto/`
  - `gen_keys.go`: Helpers to generate keypairs for nodes/peers and write them to JSON.
  - `address.go`, `keypairs.go`: Basic keypair/address utilities for signing/identity.
  - `*_test.go`: Unit tests for cryptographic helpers.

- `network/`
  - `transport.go`: Abstractions for a transport layer (send/receive primitives).
  - `libp2p_transport.go`: A libp2p-based transport implementation (currently minimal/experimental).
  - `peer.go`: Peer identity and addressing utilities.

- `rpc/`
  - `rpc.go`, `codec.go`, `json_codec.go`: Minimal RPC façade and JSON codec for message serialization.
  - `message.go`: Message types and envelopes exchanged across the RPC layer.
  - `pacemaker_rpc.go`, `reporter_rpc.go`, `packermaker.go`: RPC endpoints/types for OCR roles (naming is WIP).

- `ocr/`
  - `ocr.go`, `ocr_builder.go`: OCR node composition and lifecycle management.
  - `server.go`, `server_rpc_processor.go`: In-process server shell plus RPC plumbing for OCR messages.
  - `leader.go`, `leader_selection.go`: Leader role and basic leader selection strategies.
  - `pacemaker.go`, `timer.go`: Pacemaker that drives rounds/epochs and timeouts.
  - `messaging_layer.go`: Internal messaging façade for OCR components.
  - `observation_map.go`, `report.go`, `report_assembler.go`: Handling observations, aggregation, and report assembly.
  - `reporter.go`, `transmitter.go`: Reporting pipeline and publication/transmission stub.
  - `utils.go`: OCR-specific utilities.
  - `jobs/`: Job ingestion, observation, and direct requests
    - `job.go`, `job_source.go`: Job model and source abstraction
    - `job_reader.go`, `job_reader_test.go`: Parse TOML jobs from disk
    - `job_observation.go`: Turn jobs into observations
    - `direct_req.go`: Example direct-request style job

- `jobs_def/`
  - Example TOML job definitions used by the OCR jobs pipeline during testing/development (`job1.toml`, `job2.toml`, `job3.toml`).

- `logger/`
  - `logger.go`: Thin wrapper around `zap` for structured logging.

- `utils/`
  - `common.go`: Shared helpers not specific to OCR.

- `bin/`
  - Build artifacts destination; `make build` produces `bin/eavesdrop`.


## How the pieces fit together (conceptual)
1. Startup
   - `main.go` creates `ocr.Server` instances with a codec and private keys, then wraps them in `ocr.OCR` nodes and starts them.
2. Pacemaker & leader selection
   - `ocr.Pacemaker` drives time/rounds and facilitates leader rotation using simple strategies in `leader_selection.go`.
3. Observations & jobs
   - The jobs subsystem reads example TOML jobs, produces observations, and feeds them into the OCR pipeline.
4. Reporting & transmission
   - Observations are aggregated into a report via `report_assembler.go`, then handed off to `reporter.go`/`transmitter.go`.
5. Messaging & RPC
   - Components communicate through a light RPC façade (`rpc/`) with JSON codec and optional transport abstractions (`network/`).

This demo runs entirely in-process by default; networking and persistence are intentionally minimal.


## Configuration and keys
- Keys
  - By default, `main.go` generates ephemeral private keys via `crypto.GeneratePrivateKey()` for each server instance.
  - You can optionally generate key pairs to `peers.json` for experimentation:

    ```bash
    go test ./crypto -run TestGenerateKeyPairsJSON -v | true
    ```

    Or call the helper in a custom main by invoking `crypto.GenerateKeyPairsJSON(count, path)`.

- Jobs
  - Example jobs live under `jobs_def/` and are parsed by the OCR jobs reader. Adjust or add TOMLs to experiment with different observations.


## Development
- Linting/formatting: standard Go tooling (`gofmt`, `go vet`) is recommended
- Testing: `make test` or `go test -v ./...`
- Building: `make build` or `go build -o ./bin/eavesdrop`


## Roadmap / Known gaps
- Robust leader election, view change, and fault handling
- Honest/malicious behavior modeling and byzantine resilience
- Cryptographic signatures over observations/reports and verification paths
- Persistent networking configuration and realistic libp2p integration
- Transport security, replay protection, and message authentication
- End-to-end report publication/commitment and external adapter integration
- Comprehensive metrics, tracing, and operator tooling


## License
This project is provided as-is for educational purposes. See `LICENSE` if present, or assume usage is restricted to local experimentation unless otherwise stated. 