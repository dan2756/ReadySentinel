# waitfor

Wait until a TCP port accepts connections, then exit 0.

Handy in shell scripts, Makefiles and container entrypoints where something
has to be up before the next step runs:

```sh
waitfor db:5432 && ./migrate
```

## Install

```sh
go install github.com/dan2756/waitfor@latest
```

## Usage

```
waitfor [flags] TARGET...
```

A target can be written as:

| Target            | Meaning                         |
|-------------------|---------------------------------|
| `5432`, `:5432`   | TCP port on `localhost`         |
| `host:5432`       | TCP connect to `host:5432`      |
| `tcp://host:5432` | the same, spelled out           |

With several targets, waitfor waits until all of them are ready.

| Flag                 | Default | Description                                   |
|----------------------|---------|-----------------------------------------------|
| `-t, --timeout DUR`  | `0`     | give up after DUR; `0` waits forever          |
| `-i, --interval DUR` | `500ms` | delay between attempts (backs off to 4x)      |
| `-q, --quiet`        |         | no progress output                            |
| `-h, --help`         |         | show help                                     |
| `--version`          |         | print the version                             |

Durations are Go-style (`30s`, `1m30s`, `500ms`); a bare number is seconds.

## Exit status

| Code | Meaning     |
|------|-------------|
| 0    | ready       |
| 1    | timed out   |
| 2    | usage error |

## License

MIT
