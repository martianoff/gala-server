// Loadgen fires randomly-paced HTTP requests at a local gala-server instance,
// mostly so you can stand up the //examples/dashboard TUI in one terminal and
// see the sparkline tick along while this beats on it from another.
//
// Run:    bazel run //examples/loadgen
// Custom: bazel run //examples/loadgen -- -url http://localhost:9000 -min-ms 20 -max-ms 200 -duration 30s
//
// Defaults match the routes wired up by //examples/dashboard:
//   GET /                    (200)
//   GET /hello/{name}        (200)
//   GET /slow                (200, ~150ms)
//   GET /error               (500 — exercises the dashboard's error path)
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

type route struct {
	method string
	path   string // may contain {name} placeholders
	weight int    // relative weight in the random picker
}

// Routes match examples/dashboard/main.gala. Weights are biased toward the
// "boring 200s" so the dashboard mostly looks healthy with occasional spikes.
var routes = []route{
	{method: "GET", path: "/", weight: 8},
	{method: "GET", path: "/hello/{name}", weight: 8},
	{method: "GET", path: "/slow", weight: 2},
	{method: "GET", path: "/error", weight: 1},
}

var names = []string{"alice", "bob", "carol", "dave", "eve", "frank", "grace", "heidi", "ivan", "judy"}

func pickRoute(r *rand.Rand) route {
	total := 0
	for _, rt := range routes {
		total += rt.weight
	}
	pick := r.Intn(total)
	for _, rt := range routes {
		if pick < rt.weight {
			return rt
		}
		pick -= rt.weight
	}
	return routes[0]
}

func render(rt route, r *rand.Rand) string {
	p := rt.path
	if strings.Contains(p, "{name}") {
		p = strings.ReplaceAll(p, "{name}", names[r.Intn(len(names))])
	}
	return p
}

func main() {
	var (
		baseURL  = flag.String("url", "http://localhost:8080", "base URL of the target server")
		minMs    = flag.Int("min-ms", 50, "minimum delay between requests (ms)")
		maxMs    = flag.Int("max-ms", 500, "maximum delay between requests (ms)")
		duration = flag.Duration("duration", 0, "stop after this duration (0 = run until interrupted)")
		quiet    = flag.Bool("quiet", false, "suppress per-request log lines")
		seed     = flag.Int64("seed", 0, "RNG seed (0 = use current time)")
	)
	flag.Parse()

	if *minMs < 0 || *maxMs < *minMs {
		fmt.Fprintln(os.Stderr, "loadgen: -max-ms must be >= -min-ms >= 0")
		os.Exit(2)
	}

	s := *seed
	if s == 0 {
		s = time.Now().UnixNano()
	}
	r := rand.New(rand.NewSource(s))

	*baseURL = strings.TrimRight(*baseURL, "/")
	client := &http.Client{Timeout: 10 * time.Second}

	stop := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		close(stop)
	}()

	var deadline <-chan time.Time
	if *duration > 0 {
		deadline = time.After(*duration)
	}

	fmt.Printf("loadgen: hitting %s with delays in [%dms, %dms], seed=%d\n", *baseURL, *minMs, *maxMs, s)
	if *duration > 0 {
		fmt.Printf("loadgen: will stop after %s\n", *duration)
	} else {
		fmt.Println("loadgen: ctrl+c to stop")
	}

	var sent, ok2xx, ok3xx, err4xx, err5xx, errNet uint64
	start := time.Now()

	for {
		select {
		case <-stop:
			summary(start, &sent, &ok2xx, &ok3xx, &err4xx, &err5xx, &errNet)
			return
		case <-deadline:
			summary(start, &sent, &ok2xx, &ok3xx, &err4xx, &err5xx, &errNet)
			return
		default:
		}

		rt := pickRoute(r)
		path := render(rt, r)
		url := *baseURL + path

		req, err := http.NewRequest(rt.method, url, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "loadgen: build %s %s: %v\n", rt.method, path, err)
			atomic.AddUint64(&errNet, 1)
			continue
		}
		t0 := time.Now()
		resp, err := client.Do(req)
		atomic.AddUint64(&sent, 1)
		elapsed := time.Since(t0)

		if err != nil {
			atomic.AddUint64(&errNet, 1)
			if !*quiet {
				fmt.Printf("%6s %-30s ERR %v\n", rt.method, path, err)
			}
		} else {
			resp.Body.Close()
			switch {
			case resp.StatusCode >= 200 && resp.StatusCode < 300:
				atomic.AddUint64(&ok2xx, 1)
			case resp.StatusCode >= 300 && resp.StatusCode < 400:
				atomic.AddUint64(&ok3xx, 1)
			case resp.StatusCode >= 400 && resp.StatusCode < 500:
				atomic.AddUint64(&err4xx, 1)
			default:
				atomic.AddUint64(&err5xx, 1)
			}
			if !*quiet {
				fmt.Printf("%6s %-30s %3d  %6s\n", rt.method, path, resp.StatusCode, elapsed.Round(time.Millisecond))
			}
		}

		span := *maxMs - *minMs
		var delay time.Duration
		if span <= 0 {
			delay = time.Duration(*minMs) * time.Millisecond
		} else {
			delay = time.Duration(*minMs+r.Intn(span+1)) * time.Millisecond
		}
		select {
		case <-stop:
		case <-time.After(delay):
		}
	}
}

func summary(start time.Time, sent, ok2xx, ok3xx, err4xx, err5xx, errNet *uint64) {
	dur := time.Since(start)
	s := atomic.LoadUint64(sent)
	rate := float64(s) / dur.Seconds()
	fmt.Println()
	fmt.Printf("loadgen: stopped after %s — %d requests (%.1f req/s avg)\n", dur.Round(10*time.Millisecond), s, rate)
	fmt.Printf("loadgen:   2xx=%d  3xx=%d  4xx=%d  5xx=%d  net-err=%d\n",
		atomic.LoadUint64(ok2xx),
		atomic.LoadUint64(ok3xx),
		atomic.LoadUint64(err4xx),
		atomic.LoadUint64(err5xx),
		atomic.LoadUint64(errNet),
	)
}
