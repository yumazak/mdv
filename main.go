package main

import (
	"crypto/sha1"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

//go:embed template.html
var templateHTML string

var (
	flagOut      = flag.String("o", "", "write the HTML here instead of the cache dir")
	flagNoOpen   = flag.Bool("no-open", false, "do not open the browser")
	flagTheme    = flag.String("theme", "auto", "mermaid theme: auto, dark, light")
	flagTitle    = flag.String("title", "", "document title")
	flagClean    = flag.Bool("clean", false, "remove cached previews and exit")
	flagVendor   = flag.String("vendor", "", "vendor directory (default: next to the binary)")
	flagKeepDays = flag.Int("keep-days", 7, "remove cached previews older than this many days")
)

var headingPattern = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(.+?)[ \t]*$`)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: mdv [flags] FILE.md\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}
}

func run() error {
	if *flagClean {
		n, err := cleanCache()
		if err != nil {
			return err
		}
		fmt.Printf("removed %d cached preview(s)\n", n)
		return nil
	}

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	srcPath, err := filepath.Abs(flag.Arg(0))
	if err != nil {
		return err
	}
	if _, err := os.Stat(srcPath); err != nil {
		return err
	}

	theme, err := normalizeTheme(*flagTheme)
	if err != nil {
		return err
	}

	vendor, err := vendorDir()
	if err != nil {
		return err
	}

	markdown, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	outPath := *flagOut
	if outPath == "" {
		outPath, err = cachePath(srcPath)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	page, err := buildPage(srcPath, markdown, vendor, theme)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, []byte(page), 0o644); err != nil {
		return err
	}

	if *flagKeepDays > 0 {
		pruneCache(time.Duration(*flagKeepDays) * 24 * time.Hour)
	}

	fmt.Println(outPath)

	if !*flagNoOpen {
		return openBrowser(outPath)
	}
	return nil
}

func normalizeTheme(v string) (string, error) {
	switch strings.ToLower(v) {
	case "", "auto":
		return "auto", nil
	case "dark":
		return "dark", nil
	case "light", "default":
		return "light", nil
	default:
		return "", fmt.Errorf("unknown theme %q (use auto, dark or light)", v)
	}
}

func vendorDir() (string, error) {
	dir := *flagVendor
	if dir == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		dir = filepath.Join(filepath.Dir(exe), "vendor")
	}
	required := []string{"marked.umd.js", "mermaid.min.js"}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return "", fmt.Errorf("vendor file missing: %s (run fetch-vendor.sh or pass -vendor)", filepath.Join(dir, name))
		}
	}
	return dir, nil
}

func cacheDir() (string, error) {
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Caches", "mdv"), nil
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "mdv"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "mdv"), nil
}

func cachePath(srcPath string) (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte(srcPath))
	name := hex.EncodeToString(sum[:])[:12] + "-" + filepath.Base(srcPath) + ".html"
	return filepath.Join(dir, name), nil
}

func cleanCache() (int, error) {
	dir, err := cacheDir()
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
			removed++
		}
	}
	return removed, nil
}

func pruneCache(maxAge time.Duration) {
	dir, err := cacheDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}

func buildPage(srcPath string, markdown []byte, vendor, theme string) (string, error) {
	absVendor, err := filepath.Abs(vendor)
	if err != nil {
		return "", err
	}
	dirURL := fileURL(filepath.Dir(srcPath))
	if !strings.HasSuffix(dirURL, "/") {
		dirURL += "/"
	}

	title := *flagTitle
	if title == "" {
		title = deriveTitle(srcPath, markdown)
	}

	replacements := []string{
		"{{TITLE}}", html.EscapeString(title),
		"{{THEME}}", jsString(theme),
		"{{BASE}}", jsString(dirURL),
		"{{MD}}", jsString(base64.StdEncoding.EncodeToString(markdown)),
		"{{MARKED}}", html.EscapeString(fileURL(filepath.Join(absVendor, "marked.umd.js"))),
		"{{MERMAID}}", html.EscapeString(fileURL(filepath.Join(absVendor, "mermaid.min.js"))),
	}
	return strings.NewReplacer(replacements...).Replace(templateHTML), nil
}

func deriveTitle(srcPath string, markdown []byte) string {
	if m := headingPattern.FindSubmatch(markdown); len(m) == 2 {
		if t := strings.TrimSpace(string(m[1])); t != "" {
			return t
		}
	}
	return strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
}

func fileURL(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func jsString(v string) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `""`
	}
	return string(b)
}

func openBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}
