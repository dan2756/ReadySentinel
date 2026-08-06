# waitfor

Wait for a port, URL, file, or process exit, then exit 0.

Handy in shell scripts, Makefiles and container entrypoints where something
has to be up before the next step runs:

```sh
waitfor db:5432 && ./migrate
waitfor http://localhost:8080/health -t 30s && ./smoke-test
waitfor file ./build/done && ./package
waitfor pid "$(cat server.pid)" && echo "server stopped"
```

## Install

```sh
go install github.com/dan2756/waitfor@latest
```

## Usage

```
waitfor [flags] TARGET... [-- COMMAND [ARG...]]
```

A target can be written as:

| Target                  | Ready when                              |
|-------------------------|-----------------------------------------|
| `5432`, `:5432`         | TCP port on `localhost` accepts         |
| `host:5432`             | TCP connect to `host:5432` succeeds     |
| `tcp://host:5432`       | the same, spelled out                   |
| `http://…`, `https://…` | GET returns an accepted status          |
| `file PATH`             | PATH exists                             |
| `./PATH`, `/PATH`, `a/b` | the same: anything with a `/` is a file |
| `pid PID`               | process PID has exited                  |

With several targets, waitfor waits until all of them are ready, or with
`--any`, until the first one is:

```sh
waitfor db:5432 cache:6379 search:9200            # all three
waitfor --any primary:5432 replica:5432           # whichever answers first
```

| Flag                    | Default   | Description                              |
|-------------------------|-----------|------------------------------------------|
| `-t, --timeout DUR`     | `0`       | give up after DUR; `0` waits forever     |
| `-i, --interval DUR`    | `500ms`   | delay between attempts (backs off to 4x) |
| `--attempt-timeout DUR` | `5s`      | limit for a single connect or request    |
| `--any`                 |           | done when any one target is ready        |
| `--status RANGE`        | `200-399` | accepted HTTP status: `200` or `200-299` |
| `--nonempty`            |           | file targets must have content           |
| `--gone`                |           | wait for file targets to disappear       |
| `-q, --quiet`           |           | no progress output                       |
| `-h, --help`            |           | show help                                |
| `--version`             |           | print the version                        |

Durations are Go-style (`30s`, `1m30s`, `500ms`); a bare number is seconds.

HTTP targets send a `GET` and do not follow redirects, so a `302` to a login
page counts as up under the default range. Use `--status 200` to insist on a
real `200`.

`--nonempty` treats a directory as non-empty once it has an entry. `--gone`
is handy for lock files: `waitfor --gone ./deploy.lock`.

`pid` targets poll `kill(PID, 0)`, so the process does not have to be a child
of waitfor. They are available on Linux and macOS.

## Running a command

Anything after `--` is a command to run once every target is ready:

```sh
waitfor db:5432 cache:6379 -t 60s -- npm start
```

waitfor `exec`s the command in its own place, the way a shell's `exec`
does: it keeps the same pid, stdin/stdout/stderr and environment, receives
signals directly, and its exit status is the one you see. That makes it a
good fit for container entrypoints. If the wait times out, the command is
not run.

## Exit status

| Code | Meaning                                         |
|------|-------------------------------------------------|
| 0    | ready                                           |
| 1    | timed out                                       |
| 2    | usage error                                     |
| 126  | the command after `--` could not be run         |
| 127  | the command after `--` was not found            |
| 130  | interrupted with Ctrl-C (SIGINT)                |
| 143  | terminated (SIGTERM)                            |

When a command runs after `--`, waitfor's exit status is the command's.

## License

MIT
