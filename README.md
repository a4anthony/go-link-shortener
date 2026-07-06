# go-link-shortener

High-performance, multi-tenant URL shortener + click-analytics service in Go
(Gin, pgx v5, Redis, Prometheus).

> Full documentation — architecture diagram, API reference, benchmarks, and
> design decisions — lands in the docs batch. This is the project skeleton.

## Quickstart

```bash
docker compose up --build
```

The server listens on `:8080`. In dev mode it seeds a demo tenant and prints its
API key to the logs.

## Local development

```bash
make run      # run the server
make test     # unit tests
make lint     # golangci-lint
make bench    # redirect hot-path benchmarks
```

See the [Makefile](Makefile) for the full target list.

## License

[MIT](LICENSE)
