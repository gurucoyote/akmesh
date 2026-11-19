package net

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// result is one request's outcome, kept small so thousands fit in memory.
type result struct {
	target string
	status int
	code   int
}

// runParallel drives the same requests curl -Z does: every target is fetched by
// its own goroutine, at most --parallel-max at a time, and the bodies are thrown
// away. What an agent wants from a flood is the count, the failures and the
// status codes, not 5000 pages (acmesh-730).
func runParallel(opts netOptions, targets []string, stderr *os.File) int {
	start := time.Now()
	results := make([]result, len(targets))
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		fmt.Fprintf(stderr, "curl: cannot open %s: %v\n", os.DevNull, err)
		return 2
	}
	defer devnull.Close()

	limit := opts.ParallelMax
	if limit <= 0 {
		limit = defaultParallelMax
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func(i int, target string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// A copy per request: the status sink must not be shared, or the
			// last writer wins over a hundred answers.
			one := opts
			one.URL = target
			one.Output = ""
			one.Remote = false
			one.Resume = false
			one.Include = false
			one.Verbose = false
			one.Silent = true
			one.Parallel = false
			one.Repeat = 0
			res := result{target: target}
			one.StatusSink = &res.status
			res.code = runNet(one, nil, devnull, devnull)
			results[i] = res
		}(i, target)
	}
	wg.Wait()
	elapsed := time.Since(start)

	failed := 0
	codes := map[int]int{}
	for _, r := range results {
		if r.code != 0 {
			failed++
		}
		codes[r.status]++
	}
	if !opts.Silent {
		var parts []string
		for status, n := range codes {
			if status == 0 {
				parts = append(parts, fmt.Sprintf("no answer: %d", n))
				continue
			}
			parts = append(parts, fmt.Sprintf("%d: %d", status, n))
		}
		sort.Strings(parts)
		rate := 0.0
		if s := elapsed.Seconds(); s > 0 {
			rate = float64(len(targets)) / s
		}
		fmt.Fprintf(stderr, "%d requests, %d failed, %.2f s, %.0f req/s, %s\n",
			len(targets), failed, elapsed.Seconds(), rate, strings.Join(parts, " "))
	}
	if opts.Fail {
		// --fail keeps its meaning: a 4xx or 5xx answer is a failure even when
		// the transfer itself worked, and it outranks the transport error code.
		for _, r := range results {
			if r.status >= 400 {
				return exitHTTPError
			}
		}
	}
	if failed > 0 {
		return exitConnect
	}
	return 0
}
