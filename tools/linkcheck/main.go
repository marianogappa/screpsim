package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	openbw "github.com/marianogappa/openbw-go"
)

func main() {
	list := flag.String("list", "", "file with one replay path per line, relative to $OPENBW_REPLAYS ('#' starts a comment line)")
	dataDir := flag.String("data", "", "game data directory (default $OPENBW_DATA)")
	workers := flag.Int("j", runtime.NumCPU(), "parallel games")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: linkcheck [flags] [-list replays.txt] [replay.rep|dir ...]")
		flag.PrintDefaults()
	}
	flag.Parse()
	paths, err := replayPaths(*list, flag.Args())
	if err == nil && len(paths) == 0 {
		err = fmt.Errorf("no replays")
	}
	var data *openbw.Data
	if err == nil {
		data, err = openbw.LoadData(*dataDir)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "linkcheck:", err)
		os.Exit(2)
	}
	results := parallel(len(paths), *workers, func(i int) *result { return checkReplay(data, paths[i]) })
	if !report(results) {
		os.Exit(1)
	}
}

func replaysDir() string {
	if dir := os.Getenv("OPENBW_REPLAYS"); dir != "" {
		return dir
	}
	return "testdata/replays"
}

func replayPaths(list string, args []string) ([]string, error) {
	var paths []string
	if list != "" {
		f, err := os.Open(list)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			line := strings.TrimSpace(s.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				paths = append(paths, filepath.Join(replaysDir(), line))
			}
		}
		if err := s.Err(); err != nil {
			return nil, err
		}
	}
	for _, arg := range args {
		err := filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".rep") {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func parallel[T any](n, workers int, run func(int) T) []T {
	out := make([]T, n)
	next := make(chan int)
	var wg sync.WaitGroup
	for range max(workers, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				out[i] = run(i)
			}
		}()
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
	return out
}

func report(results []*result) bool {
	var errs, cmds, res, linked, unlinked, dropped, afterEnd, selections int
	formats := map[string]int{}
	ok := true
	for _, r := range results {
		if r.err != nil {
			errs++
			ok = false
			fmt.Printf("ERROR %s: %v\n", r.path, r.err)
			continue
		}
		formats[r.format]++
		cmds += r.cmds
		res += r.results
		linked += r.linked
		unlinked += r.unlinked
		dropped += r.dropped
		afterEnd += r.afterEnd
		selections += r.selections
		if r.failureCount > 0 {
			ok = false
			fmt.Printf("FAIL %s (%d failures)\n", r.path, r.failureCount)
			for _, f := range r.failures {
				fmt.Printf("  %s\n", f)
			}
		}
	}
	fmt.Printf("replays %d %v, errors %d\n", len(results), formats, errs)
	fmt.Printf("screp commands %d: dropped by the downgrade %d, after the end frame %d\n", cmds, dropped, afterEnd)
	fmt.Printf("command results %d: linked %d, not linked %d; selections compared %d\n", res, linked, unlinked, selections)
	for _, kind := range failureKinds(results) {
		n := 0
		for _, r := range results {
			n += r.failureByKind[kind]
		}
		fmt.Printf("check %-10s %d failures\n", kind, n)
	}
	if ok {
		fmt.Println("all checks passed")
	}
	return ok
}
