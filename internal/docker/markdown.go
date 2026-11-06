package docker

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

const siteMarkdownExt = ".md"

var markdownParser = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(goldmarkhtml.WithUnsafe()),
)

// renderMarkdownDocuments rewrites every Markdown file in the served copy as a
// rendered HTML document, keeping its path so links to the .md file still work.
func renderMarkdownDocuments(root, dest string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !isMarkdownEntry(entry) {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		return writeMarkdownPage(path, filepath.Join(dest, rel))
	})
}

// firstMarkdownEntry returns the first entry of a folder that holds nothing
// but Markdown documents. Entries must already be in display order.
func firstMarkdownEntry(entries []fs.DirEntry) (fs.DirEntry, bool) {
	if len(entries) == 0 {
		return nil, false
	}

	for _, entry := range entries {
		if !isMarkdownEntry(entry) {
			return nil, false
		}
	}

	return entries[0], true
}

// Helpers

func isMarkdownEntry(entry fs.DirEntry) bool {
	return entry.Type().IsRegular() && isMarkdownName(entry.Name())
}

func isMarkdownName(name string) bool {
	return strings.EqualFold(filepath.Ext(name), siteMarkdownExt)
}

func writeMarkdownPage(source, target string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	if err := markdownParser.Convert(content, &body); err != nil {
		return fmt.Errorf("rendering %s: %w", source, err)
	}

	page := markdownPage{
		Title: markdownTitle(content, filepath.Base(source)),
		Body:  template.HTML(body.String()),
	}

	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	return markdownTemplate.Execute(file, page)
}

func markdownTitle(content []byte, name string) string {
	doc := markdownParser.Parser().Parse(text.NewReader(content))

	title := markdownFirstHeading(doc, content)
	if title == "" {
		title = strings.TrimSuffix(name, filepath.Ext(name))
	}

	return title
}

func markdownFirstHeading(doc ast.Node, content []byte) string {
	var heading ast.Node

	ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		found, ok := node.(*ast.Heading)
		if !ok || !entering || found.Level != 1 {
			return ast.WalkContinue, nil
		}

		heading = found
		return ast.WalkStop, nil
	})

	if heading == nil {
		return ""
	}

	var title strings.Builder

	ast.Walk(heading, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		textNode, ok := node.(*ast.Text)
		if !ok || !entering {
			return ast.WalkContinue, nil
		}

		title.Write(textNode.Value(content))
		return ast.WalkContinue, nil
	})

	return strings.TrimSpace(title.String())
}

type markdownPage struct {
	Title string
	Body  template.HTML
}

var markdownTemplate = template.Must(template.New("markdown").Parse(markdownPageHTML))

const markdownPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · ONCE</title>
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
    font: 1rem/1.7 Georgia, "Iowan Old Style", "Times New Roman", serif;
  }

  main {
    max-width: 46rem;
    margin: 0 auto;
    padding: clamp(1.5rem, 4vw, 2.75rem);
    background: var(--card);
    border: 1px solid var(--rule);
    box-shadow: 0 1px 0 var(--rule);
  }

  article > :first-child { margin-top: 0; }
  article > :last-child { margin-bottom: 0; }

  h1, h2, h3, h4, h5, h6 {
    margin: 2rem 0 0.75rem;
    font-weight: 400;
    line-height: 1.2;
  }

  h1 { font-size: clamp(1.75rem, 4vw, 2.25rem); }
  h2 { font-size: 1.5rem; }
  h3 { font-size: 1.25rem; }

  h1, h2 {
    padding-bottom: 0.4rem;
    border-bottom: 1px solid var(--rule);
  }

  a { color: var(--accent); }

  p, ul, ol, blockquote, pre, table { margin: 0 0 1.1rem; }

  ul, ol { padding-left: 1.4rem; }

  blockquote {
    margin-left: 0;
    padding: 0.1rem 0 0.1rem 1rem;
    border-left: 3px solid var(--rule);
    color: var(--muted);
  }

  code, pre {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.875em;
  }

  code {
    padding: 0.1em 0.3em;
    background: var(--paper);
    border: 1px solid var(--rule);
  }

  pre {
    padding: 0.9rem 1rem;
    overflow-x: auto;
    background: var(--paper);
    border: 1px solid var(--rule);
  }

  pre code {
    padding: 0;
    background: none;
    border: 0;
  }

  table {
    width: 100%;
    border-collapse: collapse;
  }

  th, td {
    padding: 0.5rem 0.6rem;
    text-align: left;
    border-bottom: 1px solid var(--rule);
  }

  th {
    font-size: 0.75rem;
    font-weight: 400;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--muted);
  }

  hr {
    margin: 2rem 0;
    border: 0;
    border-top: 1px solid var(--rule);
  }

  img { max-width: 100%; height: auto; }

  footer {
    margin-top: 2rem;
    padding-top: 0.75rem;
    border-top: 1px solid var(--rule);
    font: 0.6875rem/1.5 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--muted);
  }
</style>
</head>
<body>
<main>
<article>
{{.Body}}</article>
<footer>ONCE · Markdown</footer>
</main>
</body>
</html>
`
