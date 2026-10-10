package main

import (
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)



func main() {
	port := flag.Int("port", 8080, "port to listen on")
	dir := flag.String("dir", ".", "path to serve")
	flag.Parse()

	root, err := filepath.Abs(*dir)
	if err != nil {
		fatalf("resolve dir: %v", err)
	}

	info, err := os.Stat(root)
	if err != nil {
		fatalf("stat dir: %v", err)
	}
	if !info.IsDir() {
		fatalf("%s is not a directory", root)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fatalf("create watcher: %v", err)
	}
	defer watcher.Close()

	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			logf(yellow, "WARN", "walk error at %s: %v", p, walkErr)
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if p != root && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if addErr := watcher.Add(p); addErr != nil {
			logf(yellow, "WARN", "watch %s: %v", p, addErr)
		}
		return nil
	})
	if err != nil {
		fatalf("walk: %v", err)
	}

	bc := newBroadcaster()

	go func() {
		var timer *time.Timer
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				rel, relErr := filepath.Rel(root, event.Name)
				if relErr != nil {
					rel = event.Name
				}
				logf(opColor(event), event.Op.String(), "%s", rel)

				if event.Has(fsnotify.Create) {
					if fi, statErr := os.Stat(event.Name); statErr == nil && fi.IsDir() {
						if addErr := watcher.Add(event.Name); addErr != nil {
							logf(yellow, "WARN", "watch %s: %v", rel, addErr)
						} else {
							logf(cyan, "WATCH", "%s", rel)
						}
					}
				}

				if event.Op == fsnotify.Chmod {
					continue
				}

				if timer == nil {
					timer = time.AfterFunc(Delay, func() {
						logf(cyan, "CHANGED", "reload")
						bc.broadcast()
					})
				}else {
					timer.Reset(Delay)
				}

			case watchErr, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logf(red, "ERROR", "watcher: %v", watchErr)
			}
		}
	}()

	fileServer := http.FileServer(http.Dir(root))

	mux := http.NewServeMux()
	mux.Handle("GET /events", eventHandler(bc))
	mux.Handle("GET /", inject(root, noCache(fileServer)))

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	logf(cyan, "SERVE", "%s on %s", root, paint(bold, "http://"+addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		fatalf("server: %v", err)
	}
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func eventHandler(b *broadcaster)http.HandlerFunc{
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming is not support by your server configuration", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Connection", "keep-alive")

		ch := b.add()
		defer b.remove(ch)

		fmt.Fprint(w, ": connected\n\n")
		flusher.Flush()

		for {
			select {
				case <- ch:
					fmt.Fprint(w, "data: reload\n\n")
					flusher.Flush()
				case <- r.Context().Done():
					return 
			}
		}
	}
}


func inject(root string, next http.Handler)http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(root, filepath.FromSlash(path.Clean("/"+r.URL.Path)))
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			p = filepath.Join(p, "index.html")
		}
		if !strings.HasSuffix(strings.ToLower(p), ".html") {
			next.ServeHTTP(w, r)
			return
		}
		data, err := os.ReadFile(p)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		html := string(data)
		if i := strings.LastIndex(strings.ToLower(html), "</body>"); i >= 0 {
			html = html[:i] + reloadScript + html[i:]
		} else {
			html += reloadScript
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write([]byte(html))
	})		
}
