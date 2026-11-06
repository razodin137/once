package docker

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const siteIndexName = "index.html"

// ServeSite copies the folder at source into a fresh directory in the served
// site cache and returns its path. Folders without an index.html get a
// generated contents page in the copy, so the source folder is left untouched.
func ServeSite(name, source string) (string, error) {
	dir, err := servedSiteDir(name)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating served site directory: %w", err)
	}

	revision, err := randomID(8)
	if err != nil {
		return "", fmt.Errorf("generating served site id: %w", err)
	}

	dest := filepath.Join(dir, revision)
	if err := BuildServedSite(source, dest); err != nil {
		os.RemoveAll(dest)
		return "", err
	}

	return dest, nil
}

// RemoveServedSite deletes every served copy belonging to name.
func RemoveServedSite(name string) error {
	dir, err := servedSiteDir(name)
	if err != nil {
		return err
	}

	return os.RemoveAll(dir)
}

// RemoveServedCopy deletes a single served copy, such as the previous version
// of a site after a redeploy.
func RemoveServedCopy(path string) error {
	return os.RemoveAll(path)
}

// BuildServedSite copies the folder at source into dest. Markdown files are
// rendered as HTML documents, and every folder in the copy that has no
// index.html gets a generated contents page. A folder made up entirely of
// Markdown documents opens its first one instead.
func BuildServedSite(source, dest string) error {
	root, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	if err := copyTree(root, dest); err != nil {
		return err
	}

	if err := renderMarkdownDocuments(root, dest); err != nil {
		return err
	}

	return writeContentsPages(root, dest)
}

// Helpers

func servedSiteDir(name string) (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("determining cache directory: %w", err)
	}

	return filepath.Join(cache, "once", "sites", name), nil
}

func copyTree(root, dest string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)

		switch {
		case entry.IsDir():
			return os.MkdirAll(target, 0o755)
		case entry.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case entry.Type().IsRegular():
			return copySiteFile(path, target)
		default:
			return nil
		}
	})
}

// copySiteFile writes a world-readable copy, so nginx's worker user can serve
// it whatever the permissions of the source file are.
func copySiteFile(source, target string) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}

	return dst.Close()
}

func writeContentsPages(root, dest string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() || hasSiteIndex(path) {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		return writeContentsPage(path, rel, filepath.Join(dest, rel, siteIndexName))
	})
}

func hasSiteIndex(dir string) bool {
	_, err := os.Lstat(filepath.Join(dir, siteIndexName))
	return err == nil
}

type contentsPage struct {
	Name    string
	Parent  string
	Entries []contentsEntry
	Count   string
}

type contentsEntry struct {
	Name     string
	Href     template.URL
	Dir      bool
	Size     string
	Modified string
}

func writeContentsPage(dir, rel, target string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	slices.SortFunc(entries, compareEntries)

	if first, ok := firstMarkdownEntry(entries); ok {
		return writeMarkdownPage(filepath.Join(dir, first.Name()), target)
	}

	page := contentsPage{
		Name:    filepath.Base(dir),
		Entries: make([]contentsEntry, 0, len(entries)),
	}

	if rel != "." {
		page.Parent = "../"
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return err
		}

		size := "—"
		if !entry.IsDir() {
			size = formatSiteSize(info.Size())
		}

		page.Entries = append(page.Entries, contentsEntry{
			Name:     entry.Name(),
			Href:     siteEntryHref(entry),
			Dir:      entry.IsDir(),
			Size:     size,
			Modified: info.ModTime().Format("2006-01-02 15:04"),
		})
	}

	page.Count = formatEntryCount(len(page.Entries))

	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	return contentsTemplate.Execute(file, page)
}

func compareEntries(a, b fs.DirEntry) int {
	if a.IsDir() != b.IsDir() {
		if a.IsDir() {
			return -1
		}
		return 1
	}

	if result := strings.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name())); result != 0 {
		return result
	}

	return strings.Compare(a.Name(), b.Name())
}

func siteEntryHref(entry fs.DirEntry) template.URL {
	href := url.PathEscape(entry.Name())
	if entry.IsDir() {
		href += "/"
	}

	return template.URL(href)
}

func formatSiteSize(size int64) string {
	const unit = 1000

	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div, exp := int64(unit), 0
	for size >= div*unit && exp < 4 {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "kMGTP"[exp])
}

func formatEntryCount(count int) string {
	if count == 1 {
		return "1 entry"
	}

	return fmt.Sprintf("%d entries", count)
}

var contentsTemplate = template.Must(template.New("contents").Parse(contentsPageHTML))

const contentsPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Name}} · ONCE</title>
<style>
  :root {
    color-scheme: light;
    --paper: #f3ede1;
    --card: #faf7f0;
    --ink: #201d19;
    --muted: #7d746a;
    --rule: #ddd3c3;
    --accent: #b4552f;
  }

  * { box-sizing: border-box; }

  body {
    margin: 0;
    padding: clamp(1.5rem, 5vw, 4rem) 1.25rem;
    background: var(--paper);
    color: var(--ink);
    font: 0.9375rem/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  }

  main {
    max-width: 48rem;
    margin: 0 auto;
    padding: clamp(1.5rem, 4vw, 2.5rem);
    background: var(--card);
    border: 1px solid var(--rule);
    box-shadow: 0 1px 0 var(--rule);
  }

  header {
    padding-bottom: 1rem;
    border-bottom: 1px solid var(--rule);
  }

  .eyebrow {
    margin: 0;
    font-size: 0.6875rem;
    letter-spacing: 0.16em;
    text-transform: uppercase;
    color: var(--muted);
  }

  h1 {
    margin: 0.35rem 0 0;
    font: 400 clamp(1.5rem, 4vw, 2rem)/1.15 Georgia, "Iowan Old Style", "Times New Roman", serif;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    margin-top: 1.25rem;
  }

  th {
    padding: 0 0.5rem 0.5rem;
    font-size: 0.6875rem;
    font-weight: 400;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    text-align: left;
    color: var(--muted);
    border-bottom: 1px solid var(--rule);
  }

  td {
    padding: 0.6rem 0.5rem;
    border-bottom: 1px solid var(--rule);
  }

  tbody tr:last-child td { border-bottom: 0; }

  .name a { color: var(--ink); text-decoration: none; }
  .name a:hover, .name a:focus { color: var(--accent); }
  .dir a::after { content: "/"; color: var(--accent); }

  .size, .date {
    color: var(--muted);
    white-space: nowrap;
    font-variant-numeric: tabular-nums;
  }

  .size { text-align: right; }
  .empty { color: var(--muted); }

  footer {
    margin-top: 1.25rem;
    font-size: 0.6875rem;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--muted);
  }
</style>
</head>
<body>
<main>
  <header>
    <p class="eyebrow">ONCE · Contents</p>
    <h1>{{.Name}}</h1>
  </header>
  <table>
    <thead>
      <tr><th scope="col">Name</th><th scope="col">Size</th><th scope="col">Modified</th></tr>
    </thead>
    <tbody>
      {{if .Parent}}<tr><td class="name"><a href="{{.Parent}}">..</a></td><td class="size">—</td><td class="date">—</td></tr>{{end}}
      {{range .Entries}}<tr><td class="name{{if .Dir}} dir{{end}}"><a href="{{.Href}}">{{.Name}}</a></td><td class="size">{{.Size}}</td><td class="date">{{.Modified}}</td></tr>{{end}}
      {{if not .Entries}}<tr><td class="empty" colspan="3">This folder is empty.</td></tr>{{end}}
    </tbody>
  </table>
  <footer>{{.Count}}</footer>
</main>
</body>
</html>
`
