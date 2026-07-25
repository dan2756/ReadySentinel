# waitfor

Wait until a TCP port accepts connections or a URL answers, then exit 0.

Handy in shell scripts, Makefiles and container entrypoints where something
has to be up before the next step runs:

```sh
waitfor db:5432 && ./migrate
waitfor http://localhost:8080/health -t 30s && ./smoke-test
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
| `http://…`, `https://…` | GET returns an accepted status |

With several targets, waitfor waits until all of them are ready.

| Flag                 | Default | Description                                   |
|----------------------|---------|-----------------------------------------------|
| `-t, --timeout DUR`  | `0`     | give up after DUR; `0` waits forever          |
| `-i, --interval DUR` | `500ms` | delay between attempts (backs off to 4x)      |
| `--attempt-timeout DUR` | `5s` | limit for a single connect or request        |
| `--status RANGE`     | `200-399` | accepted HTTP status: `200` or `200-299`    |
| `-q, --quiet`        |         | no progress output                            |
| `-h, --help`         |         | show help                                     |
| `--version`          |         | print the version                             |

Durations are Go-style (`30s`, `1m30s`, `500ms`); a bare number is seconds.

HTTP targets send a `GET` and do not follow redirects, so a `302` to a login
page counts as up under the default range. Use `--status 200` to insist on a
real `200`.

## Exit status

| Code | Meaning     |
|------|-------------|
| 0    | ready       |
| 1    | timed out   |
| 2    | usage error |

## License

MIT
