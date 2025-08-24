package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"unicode"
)

type LinkIndex map[string][]string

var (
	dryRun  = false
	verbose = true
)

func main() {
	contentDir := "content"
	index := buildIndex(contentDir)

	// Regex untuk wikilink [[Page#Fragment|Alias]]
	wikiLinkRegex := regexp.MustCompile(`\[\[([^|\]#]+)(?:#([^\]|]+))?(?:\|([^\]]+))?\]\]`)

	fileChan := make(chan string, 100)
	var wg sync.WaitGroup
	numWorkers := runtime.NumCPU() // otomatis sesuai jumlah core

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range fileChan {
				if err := processFile(path, wikiLinkRegex, index, contentDir); err != nil {
					fmt.Printf("❌ %s: %v\n", path, err)
				} else if verbose {
					fmt.Printf("✅ %s processed\n", path)
				}
			}
		}()
	}

	// Scan semua file markdown
	filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}
		fileChan <- path
		return nil
	})

	close(fileChan)
	wg.Wait()
	fmt.Println("🚀 Processing done")
}

func buildIndex(contentDir string) LinkIndex {
	index := make(LinkIndex)
	filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(contentDir, path)
		urlPath := "/" + strings.TrimSuffix(filepath.ToSlash(rel), ".md")
		slug := slugify(strings.TrimSuffix(info.Name(), ".md"))

		index[slug] = append(index[slug], urlPath)
		cleanPath := strings.ToLower(strings.TrimSuffix(filepath.ToSlash(rel), ".md"))
		index[cleanPath] = append(index[cleanPath], urlPath)
		if strings.HasPrefix(cleanPath, "/") {
			index[cleanPath[1:]] = append(index[cleanPath[1:]], urlPath)
		}
		return nil
	})
	return index
}

func processFile(path string, wikiLinkRegex *regexp.Regexp, index LinkIndex, contentDir string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	original := string(content)

	currentDir := filepath.Dir(path)
	relCurrentDir, _ := filepath.Rel(contentDir, currentDir)

	newContent := wikiLinkRegex.ReplaceAllStringFunc(original, func(match string) string {
		sub := wikiLinkRegex.FindStringSubmatch(match)
		linkName := strings.TrimSpace(sub[1])
		fragment := ""
		if sub[2] != "" {
			fragment = "#" + sub[2]
		}
		displayText := linkName
		if sub[3] != "" {
			displayText = sub[3]
		}

		url := resolveLink(linkName, index, path, relCurrentDir)
		if url == "" {
			if verbose {
				fmt.Printf("⚠️ Broken link detected: '%s' in file %s\n", linkName, path)
			}
			return fmt.Sprintf(`<span class="broken-link">%s</span>`, displayText)
		}
		return fmt.Sprintf("[%s](%s%s)", displayText, url, fragment)
	})

	if !dryRun && newContent != original {
		err = os.WriteFile(path, []byte(newContent), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func resolveLink(linkName string, index LinkIndex, currentFile, relCurrentDir string) string {
	var url string
	lowerLink := strings.ToLower(linkName)
	if strings.HasPrefix(linkName, "/") {
		clean := strings.TrimPrefix(filepath.ToSlash(lowerLink), "/")
		url = findBestMatch(clean, index, currentFile)
	} else if strings.Contains(linkName, "/") {
		fullPath := filepath.Join(relCurrentDir, linkName)
		cleanPath := strings.ToLower(strings.TrimSuffix(filepath.ToSlash(fullPath), ".md"))
		if candidates, ok := index[cleanPath]; ok && len(candidates) > 0 {
			url = candidates[0]
		}
	}
	if url == "" {
		slug := slugify(linkName)
		if candidates, ok := index[slug]; ok && len(candidates) > 0 {
			url = candidates[0]
			if len(candidates) > 1 && verbose {
				fmt.Printf("⚠️ Ambiguous link '%s' in %s → picked %s (candidates: %v)\n",
					linkName, currentFile, url, candidates)
			}
		}
	}
	return url
}

func findBestMatch(key string, index LinkIndex, currentFile string) string {
	if candidates, ok := index[key]; ok && len(candidates) > 0 {
		return candidates[0]
	}

	parts := strings.Split(key, "/")
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		if candidates, ok := index[last]; ok && len(candidates) > 0 {
			return candidates[0]
		}
	}

	slug := slugify(key)
	if candidates, ok := index[slug]; ok && len(candidates) > 0 {
		if len(candidates) > 1 && verbose {
			fmt.Printf("⚠️ Ambiguous link '%s' in %s → picked %s (candidates: %v)\n",
				key, currentFile, candidates[0], candidates)
		}
		return candidates[0]
	}

	base := filepath.Base(key)
	if base != key {
		if candidates, ok := index[base]; ok && len(candidates) > 0 {
			return candidates[0]
		}
		slugBase := slugify(base)
		if candidates, ok := index[slugBase]; ok && len(candidates) > 0 {
			return candidates[0]
		}
	}

	return ""
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		} else if unicode.IsSpace(r) || r == '-' {
			return '-'
		}
		return -1
	}, s)
	s = regexp.MustCompile(`-+`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
