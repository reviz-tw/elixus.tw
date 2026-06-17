// Command editor is a LOCAL-ONLY markdown editor for the Elixus blog.
//
// It binds to 127.0.0.1 only and is intentionally NOT part of the Hugo
// build, so it never gets deployed to GitHub Pages. Run it with `make edit`
// (or `go run .` from this directory) while you write, then deploy the
// generated static site without it.
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

//go:embed editor.html
var assets embed.FS

// All file operations are confined to these directories.
//   postsDir   = content/posts  (section "posts")
//   contentDir = content         (section "pages", e.g. about.md)
var (
	postsDir   string
	contentDir string
)

func main() {
	addr := flag.String("addr", "127.0.0.1:1314", "address to bind (localhost only by design)")
	root := flag.String("root", "", "blog root (defaults to two levels up from this tool)")
	flag.Parse()

	// Resolve blog root. Default: tools/editor -> ../../
	blogRoot := *root
	if blogRoot == "" {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("cannot determine working dir: %v", err)
		}
		// If run from tools/editor, go up two; otherwise assume cwd is root.
		if filepath.Base(wd) == "editor" {
			blogRoot = filepath.Join(wd, "..", "..")
		} else {
			blogRoot = wd
		}
	}
	abs, err := filepath.Abs(blogRoot)
	if err != nil {
		log.Fatalf("cannot resolve blog root: %v", err)
	}
	contentDir = filepath.Join(abs, "content")
	postsDir = filepath.Join(contentDir, "posts")
	if err := os.MkdirAll(postsDir, 0o755); err != nil {
		log.Fatalf("cannot create posts dir %s: %v", postsDir, err)
	}

	// Refuse to bind anything but loopback — this editor must stay local.
	host := strings.Split(*addr, ":")[0]
	if host != "127.0.0.1" && host != "localhost" {
		log.Fatalf("refusing to bind %q: editor is local-only, use 127.0.0.1", *addr)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/posts", handleList)
	mux.HandleFunc("/api/post", handleLoad)
	mux.HandleFunc("/api/save", handleSave)

	log.Printf("Elixus editor → http://%s", *addr)
	log.Printf("posts: %s", postsDir)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, err := assets.ReadFile("editor.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b)
}

type postMeta struct {
	Section  string   `json:"section"`
	File     string   `json:"file"`
	Title    string   `json:"title"`
	Date     string   `json:"date"`
	Draft    bool     `json:"draft"`
	Tags     []string `json:"tags"`
	Subtitle string   `json:"subtitle"`
}

// sectionDir maps a section name to its directory. Pages live directly in
// content/, posts in content/posts/.
func sectionDir(section string) (string, error) {
	switch section {
	case "posts":
		return postsDir, nil
	case "pages":
		return contentDir, nil
	default:
		return "", fmt.Errorf("unknown section %q", section)
	}
}

func handleList(w http.ResponseWriter, r *http.Request) {
	var items []postMeta
	items = append(items, listSection("posts", postsDir, false)...)
	items = append(items, listSection("pages", contentDir, true)...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Section != items[j].Section {
			return items[i].Section < items[j].Section // pages before posts
		}
		return items[i].Date > items[j].Date
	})
	writeJSON(w, items)
}

// listSection reads .md files directly inside dir (never recursing into
// subdirectories). When topLevelOnly is set, _index.md is skipped — those
// are section index pages, not standalone content.
func listSection(section, dir string, topLevelOnly bool) []postMeta {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []postMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if topLevelOnly && e.Name() == "_index.md" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		fm, _ := parseFrontMatter(string(b))
		fm.Section = section
		fm.File = e.Name()
		out = append(out, fm)
	}
	return out
}

type postPayload struct {
	Section  string   `json:"section"`
	File     string   `json:"file"`
	Title    string   `json:"title"`
	Date     string   `json:"date"`
	Draft    bool     `json:"draft"`
	Tags     []string `json:"tags"`
	Subtitle string   `json:"subtitle"`
	Body     string   `json:"body"`
}

func handleLoad(w http.ResponseWriter, r *http.Request) {
	section := r.URL.Query().Get("section")
	name := r.URL.Query().Get("file")
	path, err := safePath(section, name)
	if err != nil {
		writeErr(w, err)
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		writeErr(w, err)
		return
	}
	fm, body := parseFrontMatter(string(b))
	writeJSON(w, postPayload{
		Section:  section,
		File:     name,
		Title:    fm.Title,
		Date:     fm.Date,
		Draft:    fm.Draft,
		Tags:     fm.Tags,
		Subtitle: fm.Subtitle,
		Body:     body,
	})
}

func handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var p postPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeErr(w, err)
		return
	}
	if strings.TrimSpace(p.File) == "" {
		writeErr(w, fmt.Errorf("檔名不可為空"))
		return
	}
	if !strings.HasSuffix(p.File, ".md") {
		p.File += ".md"
	}
	path, err := safePath(p.Section, p.File)
	if err != nil {
		writeErr(w, err)
		return
	}
	if strings.TrimSpace(p.Date) == "" {
		p.Date = time.Now().Format("2006-01-02")
	}
	content := composeFrontMatter(p) + "\n" + strings.TrimRight(p.Body, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]string{"section": p.Section, "file": p.File, "status": "saved"})
}

// safePath resolves name relative to the section's directory and rejects
// anything that escapes it (path traversal guard).
func safePath(section, name string) (string, error) {
	dir, err := sectionDir(section)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("missing file name")
	}
	// only allow a bare filename, no separators
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid file name %q", name)
	}
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	abs, err := filepath.Abs(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(abs, dir+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes %s dir", section)
	}
	return abs, nil
}

var (
	reFMBlock = regexp.MustCompile(`(?s)^\+\+\+\s*\n(.*?)\n\+\+\+\s*\n?`)
)

// parseFrontMatter extracts the TOML +++ front matter (minimal parser for the
// fields this editor manages) and returns the remaining body.
func parseFrontMatter(s string) (postMeta, string) {
	var fm postMeta
	m := reFMBlock.FindStringSubmatch(s)
	if m == nil {
		return fm, s
	}
	block := m[1]
	body := s[len(m[0]):]
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "title":
			fm.Title = unquote(v)
		case "date":
			fm.Date = trimDate(unquote(v))
		case "draft":
			fm.Draft = v == "true"
		case "subtitle":
			fm.Subtitle = unquote(v)
		case "tags":
			fm.Tags = parseTags(v)
		}
	}
	return fm, body
}

func composeFrontMatter(p postPayload) string {
	var b strings.Builder
	b.WriteString("+++\n")
	fmt.Fprintf(&b, "title = %q\n", p.Title)
	fmt.Fprintf(&b, "date = %s\n", p.Date)
	fmt.Fprintf(&b, "draft = %t\n", p.Draft)
	b.WriteString("tags = [")
	for i, t := range p.Tags {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q", t)
	}
	b.WriteString("]\n")
	if strings.TrimSpace(p.Subtitle) != "" {
		fmt.Fprintf(&b, "subtitle = %q\n", p.Subtitle)
	}
	b.WriteString("+++\n")
	return b.String()
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// trimDate keeps only the YYYY-MM-DD part of a date value.
func trimDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func parseTags(v string) []string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	var out []string
	for _, part := range strings.Split(v, ",") {
		t := unquote(strings.TrimSpace(part))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
