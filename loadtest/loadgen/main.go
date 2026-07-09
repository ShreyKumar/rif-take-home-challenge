// Command loadgen is a small, dependency-free (stdlib-only) closed-loop load
// generator for the mutant-detector service. For each requested concurrency
// level it runs a fixed-duration burst of a realistic request mix
// (mutant POST / human POST / stats GET), then reports throughput, latency
// percentiles, and error rate — one row per level.
//
// It exists because the project deliberately avoids external tooling; a k6
// script (../mutant.js) is provided as the documented alternative. Run via
// `make loadtest` (which builds the server, starts it, and points this here).
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Reference payloads (requirements §3): one known mutant, one known human.
// Reusing the same two DNAs keeps the DB tiny (dedup) so the load exercises the
// hot path — validate → detect → dedup lookup → counter — rather than disk growth.
const (
	mutantBody = `{"dna":["ATGCGA","CAGTGC","TTATGT","AGAAGG","CCCCTA","TCACTG"]}`
	humanBody  = `{"dna":["ATGC","GCAT","TACG","CGTA"]}`
)

func main() {
	base := flag.String("url", "http://127.0.0.1:8080", "base URL of the running server")
	levelsCSV := flag.String("levels", "1,8,32,64,128", "comma-separated concurrency levels")
	perLevel := flag.Duration("duration", 3*time.Second, "load duration per concurrency level")
	mix := flag.String("mix", "mixed", "request mix: mixed (90% write / 10% stats), writes, or reads")
	flag.Parse()

	if *mix != "mixed" && *mix != "writes" && *mix != "reads" {
		fmt.Fprintf(os.Stderr, "invalid -mix %q: want mixed|writes|reads\n", *mix)
		os.Exit(2)
	}

	levels, err := parseLevels(*levelsCSV)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid -levels: %v\n", err)
		os.Exit(2)
	}

	if err := waitReady(*base, 10*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "server not ready at %s: %v\n", *base, err)
		os.Exit(1)
	}

	fmt.Printf("# Load test against %s — mix=%s, %s per level\n", *base, *mix, *perLevel)
	fmt.Printf("%-6s %10s %10s %10s %10s %8s\n", "conc", "requests", "req/s", "p50(ms)", "p99(ms)", "err%")
	for _, c := range levels {
		r := runLevel(*base, c, *perLevel, *mix)
		fmt.Printf("%-6d %10d %10.0f %10.2f %10.2f %8.2f\n",
			c, r.total, r.rps, ms(r.p50), ms(r.p99), r.errPct)
	}
}

type levelResult struct {
	total    int
	rps      float64
	p50, p99 time.Duration
	errPct   float64
}

// runLevel drives `conc` workers in a closed loop for `dur`, cycling through the
// request mix, and aggregates latencies + errors.
func runLevel(base string, conc int, dur time.Duration, mix string) levelResult {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        conc * 2,
			MaxIdleConnsPerHost: conc * 2,
		},
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		lat      []time.Duration
		errs     int64
		deadline = time.Now().Add(dur)
	)

	wg.Add(conc)
	for i := 0; i < conc; i++ {
		go func(worker int) {
			defer wg.Done()
			local := make([]time.Duration, 0, 1024)
			step := worker
			for time.Now().Before(deadline) {
				method, path, body := pick(step, mix)
				step++
				start := time.Now()
				if ok := doRequest(client, method, base+path, body); !ok {
					atomic.AddInt64(&errs, 1)
				}
				local = append(local, time.Since(start))
			}
			mu.Lock()
			lat = append(lat, local...)
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	elapsed := dur.Seconds()
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	total := len(lat)
	res := levelResult{total: total}
	if elapsed > 0 {
		res.rps = float64(total) / elapsed
	}
	if total > 0 {
		res.p50 = percentile(lat, 0.50)
		res.p99 = percentile(lat, 0.99)
		res.errPct = float64(errs) / float64(total) * 100
	}
	return res
}

// pick returns the next request for the given mix:
//   - "reads":  only GET /stats/ (exercises the O(1) counter path);
//   - "writes": alternating mutant/human POST /mutant/;
//   - "mixed":  ~45% mutant POST, ~45% human POST, ~10% stats GET.
func pick(step int, mix string) (method, path, body string) {
	switch mix {
	case "reads":
		return http.MethodGet, "/stats/", ""
	case "writes":
		if step%2 == 0 {
			return http.MethodPost, "/mutant/", mutantBody
		}
		return http.MethodPost, "/mutant/", humanBody
	default: // mixed
		if step%10 == 9 {
			return http.MethodGet, "/stats/", ""
		}
		if step%2 == 0 {
			return http.MethodPost, "/mutant/", mutantBody
		}
		return http.MethodPost, "/mutant/", humanBody
	}
}

// doRequest sends one request and reports whether it completed with a
// non-5xx / non-transport-error status. 200/403 (mutant/human) are successes.
func doRequest(client *http.Client, method, url, body string) bool {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return false
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode < 500
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)-1))
	return sorted[idx]
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func parseLevels(csv string) ([]int, error) {
	parts := strings.Split(csv, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 1 {
			return nil, fmt.Errorf("bad level %q", p)
		}
		out = append(out, n)
	}
	return out, nil
}

func waitReady(base string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/healthz")
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out after %s", timeout)
}
