// Command doc-link-check verifies that relative Markdown links resolve to files
// in the repository. HTTP links and in-page anchors are intentionally skipped.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var markdownLink = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: doc-link-check <file-or-directory>...")
		os.Exit(2)
	}

	files, err := markdownFiles(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var failures []string
	for _, file := range files {
		broken, err := brokenLinks(file)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", file, err))
			continue
		}
		failures = append(failures, broken...)
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		for _, failure := range failures {
			fmt.Fprintln(os.Stderr, failure)
		}
		os.Exit(1)
	}
	fmt.Printf("verified relative links in %d Markdown files\n", len(files))
}

func markdownFiles(paths []string) ([]string, error) {
	seen := make(map[string]struct{})
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if strings.EqualFold(filepath.Ext(path), ".md") {
				seen[filepath.Clean(path)] = struct{}{}
			}
			continue
		}
		err = filepath.WalkDir(path, func(candidate string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(candidate), ".md") {
				seen[filepath.Clean(candidate)] = struct{}{}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	files := make([]string, 0, len(seen))
	for file := range seen {
		files = append(files, file)
	}
	sort.Strings(files)
	return files, nil
}

func brokenLinks(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var failures []string
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		for _, match := range markdownLink.FindAllStringSubmatch(scanner.Text(), -1) {
			target := strings.Trim(strings.TrimSpace(match[1]), "<>")
			if index := strings.IndexByte(target, '#'); index >= 0 {
				target = target[:index]
			}
			if target == "" || strings.HasPrefix(target, "#") || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			if _, err := os.Stat(filepath.Clean(filepath.Join(filepath.Dir(path), target))); err != nil {
				failures = append(failures, fmt.Sprintf("%s:%d: unresolved link %q", path, line, match[1]))
			}
		}
	}
	return failures, scanner.Err()
}
