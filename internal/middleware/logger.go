package middleware

import (
	"bufio"
	"errors"
	"log"
	"net"
	"net/http"
	"time"
)

// Logger logs the method, path, status, and duration of each request.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		if r.URL.Path == "/ws" {
			if sw.status == http.StatusSwitchingProtocols {
				log.Printf("%s %s -> WS CONNECTED", r.Method, r.URL.Path)
			} else {
				log.Printf("%s %s -> WS FAILED (%d)", r.Method, r.URL.Path, sw.status)
			}
		} else {
			log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, sw.status, time.Since(start))
		}
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(status int) {
	sw.status = status
	sw.ResponseWriter.WriteHeader(status)
}

func (sw *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := sw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack not supported")
	}
	return hijacker.Hijack()
}

