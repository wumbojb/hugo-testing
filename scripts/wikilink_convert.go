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

	"gopkg.in/yaml.v3"
)

type LinkIndex map[string][]string

var (
	dryRun  = false
	verbose = true
)

// Struct minimal untuk baca module.mounts Hugo
type Mount struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

type Module struct {
	Mounts []Mount `yaml:"mounts"`
}

type HugoConfig struct {
	Module Module `yaml:"module"`
}

func main() {
	sources := getHugoMountSources()
	if len(sources) == 0 {
		sources = []string{"content"}
	}

	index := buildIndex(sources)

	fileChan := make(chan string, 100)
	var wg sync.WaitGroup
	numWorkers := runtime.NumCPU()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range fileChan {
				if err := processFile(path, index, sources); err != nil {
					fmt.Printf("❌ %s: %v\n", path, err)
				} else if verbose {
					fmt.Printf("✅ %s processed\n", path)
				}
			}
		}()
	}

	for _, src := range sources {
		filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				return nil
			}
			fileChan <- path
			return nil
		})
	}

	close(fileChan)
	wg.Wait()
	fmt.Println("🚀 Processing done")
}

// Ambil semua folder source yang mount ke content
func getHugoMountSources() []string {
	candidates := []string{
		"hugo.yaml",
		"config.yaml",
		"config/_default/hugo.yaml",
		"config/_default/module.yaml",
	}

	var mounts []Mount
	for _, f := range candidates {
		if _, err := os.Stat(f); err == nil {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			var cfg HugoConfig
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				continue
			}
			for _, m := range cfg.Module.Mounts {
				if m.Target == "content" {
					mounts = append(mounts, m)
				}
			}
			break
		}
	}

	var sources []string
	for _, m := range mounts {
		sources = append(sources, m.Source)
	}
	return sources
}

func buildIndex(sources []string) LinkIndex {
	index := make(LinkIndex)
	for _, src := range sources {
		filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				return nil
			}
			rel, _ := filepath.Rel(src, path)
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
	}
	return index
}

func processFile(path string, index LinkIndex, sources []string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	original := string(content)

	var contentDir string
	for _, src := range sources {
		if strings.HasPrefix(filepath.ToSlash(path), filepath.ToSlash(src)) {
			contentDir = src
			break
		}
	}

	currentDir := filepath.Dir(path)
	relCurrentDir, _ := filepath.Rel(contentDir, currentDir)

	// 1️⃣ Proses image wikilink dulu ![[...]]
	imageRegex := regexp.MustCompile(`!\[\[([^|\]]+)(?:\|([^\]]+))?\]\]`)
	newContent := imageRegex.ReplaceAllStringFunc(original, func(match string) string {
		sub := imageRegex.FindStringSubmatch(match)
		filename := strings.TrimSpace(sub[1])
		displayText := filename

		// Jangan potong kalau URL
		if !strings.HasPrefix(filename, "http://") && !strings.HasPrefix(filename, "https://") {
			if idx := strings.LastIndex(filename, "."); idx > 0 {
				displayText = filename[:idx]
			}
		}

		// Cek alias (|Title)
		if sub[2] != "" {
			displayText = sub[2]
		}

		url := resolveLink(filename, index, path, relCurrentDir)
		if url == "" {
			url = filename
		}

		return fmt.Sprintf(`![%s](%s)`, displayText, url)
	})

	// 2️⃣ Proses wikilink teks biasa [[...]]
	wikiLinkRegex := regexp.MustCompile(`\[\[([^|\]#]+)(?:#([^\]|]+))?(?:\|([^\]]+))?\]\]`)
	newContent = wikiLinkRegex.ReplaceAllStringFunc(newContent, func(match string) string {
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
