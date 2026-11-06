package htmltext

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderHeadingsLinksAndForms(t *testing.T) {
	page := `
<html>
<head><title>Ignored</title><style>p { color: red }</style></head>
<body>
<h1>Welcome</h1>
<p>Read the <a href="/about">about page</a> or <a href="https://elsewhere.test/">go elsewhere</a>.</p>
<form action="/search" method="get">
  <label for="q">Search</label>
  <input id="q" name="q" type="search">
  <input type="checkbox" name="exact" checked>
  <select name="scope"><option>all</option><option>news</option></select>
  <button>Go</button>
</form>
</body>
</html>`

	assert.Equal(t, `Welcome

Read the about page <http://example.test/about> or go elsewhere <https://elsewhere.test/>.

form GET http://example.test/search
  Search
  input "q" (search)
  [x] "exact"
  select "scope" (all, news)
  button "Go"
`, render(t, page))
}

func TestRenderLists(t *testing.T) {
	page := `<ul><li>One</li><li>Two <b>bold</b></li></ul><ol><li>First</li><li>Second</li></ol>`

	assert.Equal(t, `- One
- Two bold

1. First
2. Second
`, render(t, page))
}

func TestRenderTable(t *testing.T) {
	page := `<table><tr><th>Name</th><th>Status</th></tr><tr><td>App</td><td>running</td></tr></table>`

	assert.Equal(t, `Name | Status
App | running
`, render(t, page))
}

func TestRenderPreformattedText(t *testing.T) {
	page := "<pre>line one\n  line two</pre>"

	assert.Equal(t, "line one\n  line two\n", render(t, page))
}

func TestRenderSkipsScriptsAndHiddenFields(t *testing.T) {
	page := `<script>alert("hi")</script><form action="/login"><input type="hidden" name="token"><input name="email"></form>`

	assert.Equal(t, `form GET http://example.test/login
  input "email"
`, render(t, page))
}

func TestRenderLabelWrappingControl(t *testing.T) {
	page := `<form action="/save"><label><input type="checkbox" name="notify"> Notify me</label></form>`

	assert.Equal(t, `form GET http://example.test/save
  [ ] "notify" Notify me
`, render(t, page))
}

func TestRenderTextNodesKeepWordBoundaries(t *testing.T) {
	page := `<p><span>Hello</span> <span>world</span>, <span>again</span></p>`

	assert.Equal(t, "Hello world, again\n", render(t, page))
}

// Helpers

func render(t *testing.T, page string) string {
	t.Helper()

	base, err := url.Parse("http://example.test/")
	require.NoError(t, err)

	text, err := Render(strings.NewReader(page), base)
	require.NoError(t, err)

	return text
}
