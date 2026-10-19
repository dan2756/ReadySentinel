# waitfor

Wait for a port, URL, file, or process exit, then exit 0.

`waitfor` is a small, dependency-free replacement for the `until nc -z …;
do sleep 1; done` loops that end up in shell scripts, Makefiles, CI jobs and
container entrypoints. Point it at what has to be up (or gone) before the
next step runs:

```sh
waitfor db:5432 && ./migrate
waitfor http://localhost:8080/health -t 30s && ./smoke-test
waitfor file ./build/done && ./package
waitfor pid "$(cat server.pid)" && echo "server stopped"
waitfor db:5432 cache:6379 -- npm start
```

## Install

```sh
go install github.com/dan2756/waitfor@latest
```

## Usage

```
waitfor [flags] TARGET... [-- COMMAND [ARG...]]
```

Flags and targets can come in any order. Everything after the first `--` is
a command to run once the wait is over.

### Targets

The kind of target is worked out from how it is written:

| Target                   | Ready when                                     |
|--------------------------|------------------------------------------------|
| `5432`, `:5432`          | a TCP connection to `localhost:5432` succeeds  |
| `host:5432`, `[::1]:5432`| a TCP connection to that address succeeds      |
| `tcp://host:5432`        | the same, spelled out                          |
| `http://…`, `https://…`  | a `GET` returns an accepted status             |
| `file PATH`              | PATH exists                                    |
| `./PATH`, `/PATH`, `a/b` | the same: anything with a `/` is a file        |
| `pid PID`                | process PID has exited                         |

A bare word such as `done.flag` is ambiguous, so write `file done.flag` or
`./done.flag`.

With several targets, waitfor waits until all of them are ready, or with
`--any`, until the first one is:

```sh
waitfor db:5432 cache:6379 search:9200     # all three
waitfor --any primary:5432 replica:5432    # whichever answers first
```

### Flags

| Flag                    | Default   | Description                                  |
|-------------------------|-----------|----------------------------------------------|
| `-t, --timeout DUR`     | `0`       | give up after DUR; `0` waits forever         |
| `-i, --interval DUR`    | `500ms`   | delay between attempts (backs off to 4x)     |
| `--attempt-timeout DUR` | `5s`      | limit for a single attempt                   |
| `--any`                 |           | finish when any one target is ready          |
| `--status RANGE`        | `200-399` | accepted HTTP status: `200` or `200-299`     |
| `--contains TEXT`       |           | HTTP response body must contain TEXT         |
| `--nonempty`            |           | file targets must have content               |
| `--gone`                |           | wait for file targets to disappear instead   |
| `-q, --quiet`           |           | no progress output                           |
| `-h, --help`            |           | show help                                    |
| `--version`             |           | print the version                            |

Durations are Go-style (`30s`, `1m30s`, `500ms`); a bare number is seconds.

After each failed attempt the delay grows by half, up to four times
`--interval` (500ms, 750ms, 1.1s, 1.7s, 2s, 2s, …), so a long wait doesn't
hammer a service that is still starting. Each attempt is cut off after
`--attempt-timeout`, which keeps a firewalled host (or a file check on a
hung network mount) from stalling a poll for minutes.

`--status`, `--contains`, `--nonempty` and `--gone` apply to every target of
the matching kind.

### HTTP targets

HTTP targets send a `GET` with a `waitfor/VERSION` user agent and do not
follow redirects, so a `302` to a login page counts as up under the default
range. Use `--status 200` to insist on a real `200`, or `--status 200-299`
for any success.

`--contains` is for health endpoints that answer `200` before they are
really ready. Only the first 1 MiB of the body is searched, and it matches
as soon as TEXT arrives, even if the server is still sending the rest.

```sh
waitfor http://localhost:9200/_cluster/health --contains '"status":"green"'
```

Proxy settings come from `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY`;
`localhost` and loopback addresses are never proxied.

### File targets

A file target is ready as soon as the path exists. `--nonempty` also wants
it to have content (for a directory: at least one entry), which skips the
moment when a writer has created the file but written nothing yet. It is
ready after the first byte, though, and cannot tell when the writer is
done. To wait for a complete file, have the writer create it under another
name and rename it into place, or wait for a marker file it creates once it
has finished.

`--gone` turns it around and waits for the path to disappear, which is
handy for lock files:

```sh
waitfor --gone ./deploy.lock -- ./deploy.sh
```

A symlink counts as present for `--gone` even if what it points to doesn't
exist, so `ln -s` style locks work too.

### Process targets

`pid` targets poll `kill(PID, 0)`, so the process does not have to be a
child of waitfor. A process owned by another user still counts as running.

## Running a command

Anything after `--` is a command to run once every target is ready:

```sh
waitfor db:5432 cache:6379 -t 60s -- npm start
```

waitfor `exec`s the command in its own place, the way a shell's `exec`
does: it keeps the same pid, stdin/stdout/stderr and environment, receives
signals directly, and its exit status is the one you see. If the wait times
out, the command is not run.

## Output

Progress goes to stderr, so it never mixes with a command's stdout. With one
target it fits on a line:

```
$ waitfor localhost:5432
waiting for localhost:5432… ready after 2.4s
```

With several, each reports when it is ready. On timeout, the targets that
were not ready are listed along with why their last attempt failed:

```
$ waitfor db:5432 http://localhost:8080/health -t 30s
waiting for db:5432, http://localhost:8080/health…
db:5432 ready after 0.3s
timed out after 30.0s; not ready:
  http://localhost:8080/health: got 503 Service Unavailable
```

`-q` silences all of this; usage errors and a command that can't be started
are still reported.

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

## Examples

Container entrypoint that waits for its dependencies, then becomes the app:

```dockerfile
ENTRYPOINT ["waitfor", "db:5432", "redis:6379", "-t", "60s", "--", "./server"]
```

A CI job that starts a service in the background and waits for it:

```sh
./bin/api &
waitfor http://127.0.0.1:8080/healthz --status 200 -t 2m
go test ./e2e/...
```

A Makefile target that waits for a build step running elsewhere:

```make
# the compile job running in parallel touches build/app.done once
# build/app is complete
package:
	waitfor ./build/app.done -t 10m
	tar czf app.tgz -C build app
```

Restart a server once the old process has really exited:

```sh
kill "$(cat server.pid)"
waitfor pid "$(cat server.pid)" -t 20s && ./server
```

## Platform notes

waitfor builds for Linux and macOS. Everything works the same on both, with
one difference for `pid` targets: on Linux, a process that has exited but
not yet been reaped by its parent (a zombie) counts as exited, by checking
`/proc/PID/stat`. macOS has no `/proc`, so there such a process counts as
running until its parent waits for it.

## License

MIT
