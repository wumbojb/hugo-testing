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

// Config represents application configuration
type Config struct {
	DryRun      bool     `yaml:"dry_run"`
	Verbose     bool     `yaml:"verbose"`
	Workers     int      `yaml:"workers"`
	Sources     []string `yaml:"sources"`
	Extensions  []string `yaml:"extensions"`
	ExcludeDirs []string `yaml:"exclude_dirs"`
}

// LinkIndex maps slugs to their URL paths
type LinkIndex map[string][]string

// Mount represents a Hugo mount configuration
type Mount struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

// Module represents Hugo module configuration
type Module struct {
	Mounts []Mount `yaml:"mounts"`
}

// HugoConfig represents Hugo configuration structure
type HugoConfig struct {
	Module Module `yaml:"module"`
}

var (
	config      Config
	linkCache   sync.Map
	excludeDirs map[string]bool
)

// Regular expressions compiled at startup for better performance
var (
	wikiLinkRegex  *regexp.Regexp
	imageRegex     *regexp.Regexp
	multiDashRegex *regexp.Regexp
)

func init() {
	// Initialize regex patterns
	wikiLinkRegex = regexp.MustCompile(`\[\[([^[\]]+?)(?:#([^|\]]+))?(?:\|([^[\]]+))?\]\]`)
	imageRegex = regexp.MustCompile(`!\[\[([^[\]]+?)(?:\|([^[\]]+))?\]\]`)
	multiDashRegex = regexp.MustCompile(`-+`)

	// Default configuration
	config = Config{
		DryRun:      false,
		Verbose:     true,
		Workers:     runtime.NumCPU(),
		Extensions:  []string{".md", ".markdown"},
		ExcludeDirs: []string{".git", "node_modules", "vendor", ".obsidian"},
	}

	// Initialize exclude directories map
	excludeDirs = make(map[string]bool)
	for _, dir := range config.ExcludeDirs {
		excludeDirs[dir] = true
	}
}

func main() {
	// Load configuration from file or environment variables
	if err := loadConfig(); err != nil && config.Verbose {
		fmt.Printf("⚠️ Using default configuration: %v\n", err)
	}

	// Get content sources from Hugo configuration
	sources := getHugoMountSources()
	if len(sources) == 0 {
		sources = []string{"content"}
	}
	config.Sources = sources

	// Build index of all content files
	index := buildIndex(sources)

	// Process files
	fileChan := make(chan string, 100)
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < config.Workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for path := range fileChan {
				if err := processFile(path, index, sources); err != nil {
					fmt.Printf("❌ Worker %d: %s: %v\n", workerID, path, err)
				} else if config.Verbose {
					fmt.Printf("✅ Worker %d: %s processed\n", workerID, path)
				}
			}
		}(i)
	}

	// Walk through all source directories and send files to workers
	for _, src := range sources {
		err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("access error: %w", err)
			}

			// Skip directories in exclude list
			if info.IsDir() {
				if excludeDirs[info.Name()] {
					return filepath.SkipDir
				}
				return nil
			}

			// Check if file has a valid extension
			if !hasValidExtension(info.Name(), config.Extensions) {
				return nil
			}

			fileChan <- path
			return nil
		})

		if err != nil && config.Verbose {
			fmt.Printf("⚠️ Error walking directory %s: %v\n", src, err)
		}
	}

	close(fileChan)
	wg.Wait()

	if config.Verbose {
		fmt.Println("🚀 Processing completed successfully")
		if config.DryRun {
			fmt.Println("📝 Dry run mode - no files were modified")
		}
	}
}

// loadConfig loads configuration from file or environment variables
func loadConfig() error {
	// Try to load from config file first
	configFiles := []string{".wikilink-converter.yaml", "config.yaml", "config.yml"}

	for _, configFile := range configFiles {
		if _, err := os.Stat(configFile); err == nil {
			data, err := os.ReadFile(configFile)
			if err != nil {
				return fmt.Errorf("failed to read config file %s: %w", configFile, err)
			}

			if err := yaml.Unmarshal(data, &config); err != nil {
				return fmt.Errorf("failed to parse config file %s: %w", configFile, err)
			}

			// Update exclude directories map
			excludeDirs = make(map[string]bool)
			for _, dir := range config.ExcludeDirs {
				excludeDirs[dir] = true
			}

			if config.Verbose {
				fmt.Printf("📋 Loaded configuration from %s\n", configFile)
			}
			return nil
		}
	}

	// If no config file found, use defaults
	return fmt.Errorf("no configuration file found, using defaults")
}

// getHugoMountSources retrieves content sources from Hugo configuration
func getHugoMountSources() []string {
	candidates := []string{
		"hugo.yaml",
		"hugo.yml",
		"config.yaml",
		"config.yml",
		"config/_default/hugo.yaml",
		"config/_default/hugo.yml",
		"config/_default/config.yaml",
		"config/_default/config.yml",
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

			if len(mounts) > 0 {
				break
			}
		}
	}

	var sources []string
	for _, m := range mounts {
		sources = append(sources, m.Source)
	}

	return sources
}

// buildIndex creates an index of all content files for efficient lookup
func buildIndex(sources []string) LinkIndex {
	index := make(LinkIndex)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, src := range sources {
		wg.Add(1)
		go func(source string) {
			defer wg.Done()

			err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return fmt.Errorf("access error: %w", err)
				}

				// Skip directories in exclude list
				if info.IsDir() {
					if excludeDirs[info.Name()] {
						return filepath.SkipDir
					}
					return nil
				}

				// Check if file has a valid extension
				if !hasValidExtension(info.Name(), config.Extensions) {
					return nil
				}

				rel, err := filepath.Rel(source, path)
				if err != nil {
					return fmt.Errorf("failed to get relative path: %w", err)
				}

				urlPath := "/" + strings.TrimSuffix(filepath.ToSlash(rel), ".md")
				slug := slugify(strings.TrimSuffix(info.Name(), ".md"))
				cleanPath := strings.ToLower(strings.TrimSuffix(filepath.ToSlash(rel), ".md"))

				mu.Lock()
				index[slug] = append(index[slug], urlPath)
				index[cleanPath] = append(index[cleanPath], urlPath)

				if strings.HasPrefix(cleanPath, "/") {
					index[cleanPath[1:]] = append(index[cleanPath[1:]], urlPath)
				}
				mu.Unlock()

				return nil
			})

			if err != nil && config.Verbose {
				fmt.Printf("⚠️ Error building index for %s: %v\n", source, err)
			}
		}(src)
	}

	wg.Wait()
	return index
}

// processFile processes a single file, converting wikilinks to markdown links
func processFile(path string, index LinkIndex, sources []string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	original := string(content)

	// Determine content directory and relative path
	var contentDir string
	for _, src := range sources {
		if strings.HasPrefix(filepath.ToSlash(path), filepath.ToSlash(src)) {
			contentDir = src
			break
		}
	}

	currentDir := filepath.Dir(path)
	relCurrentDir, _ := filepath.Rel(contentDir, currentDir)

	// Process image wikilinks first ![[...]]
	newContent := imageRegex.ReplaceAllStringFunc(original, func(match string) string {
		sub := imageRegex.FindStringSubmatch(match)
		filename := strings.TrimSpace(sub[1])
		displayText := filename

		// Don't truncate URLs
		if !strings.HasPrefix(filename, "http://") && !strings.HasPrefix(filename, "https://") {
			if idx := strings.LastIndex(filename, "."); idx > 0 {
				displayText = filename[:idx]
			}
		}

		// Check for alias (|Title)
		if sub[2] != "" {
			displayText = sub[2]
		}

		url := resolveLink(filename, index, path, relCurrentDir)
		if url == "" {
			url = filename
		}

		return fmt.Sprintf(`![%s](%s)`, displayText, url)
	})

	// Process regular wikilinks [[...]]
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
			if config.Verbose {
				fmt.Printf("⚠️ Broken link detected: '%s' in file %s\n", linkName, path)
			}
			return fmt.Sprintf(`<span class="broken-link">%s</span>`, displayText)
		}
		return fmt.Sprintf("[%s](%s%s)", displayText, url, fragment)
	})

	// Write changes if not in dry run mode and content has changed
	if !config.DryRun && newContent != original {
		if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
	}

	return nil
}

// resolveLink finds the best matching URL for a given wikilink
func resolveLink(linkName string, index LinkIndex, currentFile, relCurrentDir string) string {
	cacheKey := linkName + "|" + currentFile
	if cached, found := linkCache.Load(cacheKey); found {
		return cached.(string)
	}

	var url string
	lowerLink := strings.ToLower(linkName)

	// Handle absolute paths
	if strings.HasPrefix(linkName, "/") {
		clean := strings.TrimPrefix(filepath.ToSlash(lowerLink), "/")
		clean = strings.TrimSuffix(clean, ".md")
		url = findBestMatch(clean, index, currentFile)
	} else if strings.HasPrefix(linkName, "../") || strings.HasPrefix(linkName, "./") {
		// Handle relative paths
		absPath := filepath.Clean(filepath.Join(filepath.Dir(currentFile), linkName))
		cleanPath := strings.ToLower(strings.TrimSuffix(filepath.ToSlash(absPath), ".md"))
		if candidates, ok := index[cleanPath]; ok && len(candidates) > 0 {
			url = candidates[0]
		}
	} else if strings.Contains(linkName, "/") {
		// Handle paths with directories
		fullPath := filepath.Join(relCurrentDir, linkName)
		cleanPath := strings.ToLower(strings.TrimSuffix(filepath.ToSlash(fullPath), ".md"))
		if candidates, ok := index[cleanPath]; ok && len(candidates) > 0 {
			url = candidates[0]
		}
	}

	// Fallback to slug matching
	if url == "" {
		slug := slugify(linkName)
		if candidates, ok := index[slug]; ok && len(candidates) > 0 {
			url = candidates[0]
			if len(candidates) > 1 && config.Verbose {
				fmt.Printf("⚠️ Ambiguous link '%s' in %s → picked %s (candidates: %v)\n",
					linkName, currentFile, url, candidates)
			}
		}
	}

	// Cache the result
	linkCache.Store(cacheKey, url)
	return url
}

// findBestMatch finds the best URL match for a given key
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
		if len(candidates) > 1 && config.Verbose {
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

// slugify converts a string to a URL-friendly slug
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
	s = multiDashRegex.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// hasValidExtension checks if a filename has one of the valid extensions
func hasValidExtension(filename string, extensions []string) bool {
	lowerFilename := strings.ToLower(filename)
	for _, ext := range extensions {
		if strings.HasSuffix(lowerFilename, ext) {
			return true
		}
	}
	return false
}
