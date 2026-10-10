# live-server

A small, dependency-light live-reload development server written in Go.

It serves a directory of static files over HTTP, watches that directory for
changes, and tells connected browsers to reload via
[Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events).

## Features

- **Static file server** for any directory, with `Cache-Control: no-store` so
  the browser always fetches fresh files.
- **Recursive file watching** using [fsnotify](https://github.com/fsnotify/fsnotify).
  New subdirectories are picked up automatically; `.git` and `node_modules`
  are skipped.
- **Debounced reloads**: bursts of file events (e.g. an editor saving
  several files) are collapsed into one reload after 80 ms of quiet.
  Chmod-only events are ignored.
- **SSE endpoint** at `/events` that pushes `reload` to every connected
  browser.
- **Colored, timestamped logs** for every file event. Color is disabled
  automatically when stderr isn't a terminal or when `NO_COLOR` is set.
- Binds to `127.0.0.1` only, so it isn't exposed on your network.

## Requirements

- Go 1.27 or newer

## Install

```sh
git clone https://github.com/walonCode/live-server.git
cd live-server
go build -o live-server .
```

Or install it straight onto your `PATH`:

```sh
go install .
```

## Usage

```sh
live-server [-dir <path>] [-port <port>]
```

| Flag    | Default | Description                  |
| ------- | ------- | ---------------------------- |
| `-dir`  | `.`     | Directory to serve and watch |
| `-port` | `8080`  | Port to listen on            |

Example, using the bundled sample site:

```sh
go run . -dir ./test-project -port 3000
```

Then open <http://127.0.0.1:3000>.

### Enabling reload in your pages

The server does **not** inject a script into your HTML. To get automatic
reloads, add this snippet to each page (just before `</body>` works well):

```html
<script>
  new EventSource("/events").onmessage = (e) => {
    if (e.data === "reload") location.reload();
  };
</script>
```

The browser keeps the connection open, and the page reloads whenever a watched
file changes. `EventSource` reconnects on its own if you restart the server.

## How it works

```
 file change ──► fsnotify watcher ──► 80 ms debounce ──► broadcaster
                                                             │
                                  browser ◄── "data: reload" ◄┘  (GET /events, SSE)
```

1. On startup, `main.go` walks the target directory and registers every
   subdirectory with an fsnotify watcher.
2. Each file event is logged and resets a debounce timer (`Delay` in
   `utils.go`).
3. When the timer fires, the `broadcaster` (`broadcaster.go`) signals every
   connected client. Each client has a 1-slot buffered channel, so a slow
   client never blocks the others and duplicate signals are dropped.
4. The `/events` handler streams `data: reload` to the browser, which
   reloads the page.

## Project layout

```
.
├── main.go          # flags, watcher, HTTP server, SSE handler
├── broadcaster.go   # fan-out of reload signals to connected clients
├── utils.go         # colored logging, debounce delay, helpers
└── test-project/    # sample static site for trying it out
```

## License

[MIT](LICENSE) © 2026 walonCode
