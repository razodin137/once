package docker

import (
	"html/template"
	"strings"
)

// appBarData is everything the bar over a served page needs to know.
type appBarData struct {
	Name     string
	Back     string
	Next     string
	Contents string
	Drift    []string
}

// Private

// appBar renders the bar injected over every page the site serves. The markup
// carries its own styles and script, so the page on disk is never touched. It
// is embedded in the nginx config, which means the result must stay free of
// single quotes and nginx variables.
func (a *Application) appBar() string {
	var bar strings.Builder
	_ = appBarTemplate.Execute(&bar, a.appBarData())
	return bar.String()
}

func (a *Application) appBarData() appBarData {
	data := appBarData{
		Name:     a.Settings.Name,
		Contents: "/",
	}

	back, next := a.neighborApps()
	if back != nil {
		data.Back = back.URL()
	}
	if next != nil {
		data.Next = next.URL()
	}

	for _, app := range a.runningSiblings() {
		data.Drift = append(data.Drift, app.URL())
	}

	return data
}

// neighborApps returns the running apps either side of this one by host, so
// back and next follow the same order as the dashboard. Either side wraps.
func (a *Application) neighborApps() (back, next *Application) {
	siblings := a.runningSiblings()
	if len(siblings) == 0 {
		return nil, nil
	}

	for _, app := range siblings {
		if app.Settings.Host < a.Settings.Host {
			back = app
		} else if next == nil {
			next = app
		}
	}

	if back == nil {
		back = siblings[len(siblings)-1]
	}
	if next == nil {
		next = siblings[0]
	}

	return back, next
}

func (a *Application) runningSiblings() []*Application {
	if a.namespace == nil {
		return nil
	}

	var siblings []*Application
	for _, app := range a.namespace.Applications() {
		if app.Settings.Name == a.Settings.Name || app.Settings.Host == "" || !app.Running {
			continue
		}
		siblings = append(siblings, app)
	}

	return siblings
}

// Helpers

var appBarTemplate = template.Must(template.New("appbar").Parse(appBarHTML))

const appBarHTML = `<div id="once-bar">
<span class="once-bar-name">{{.Name}}</span>
<nav class="once-bar-nav">
{{if .Back}}<a href="{{.Back}}" rel="prev">Back</a>{{end}}
{{if .Next}}<a href="{{.Next}}" rel="next">Next</a>{{end}}
<a href="{{.Contents}}">Contents</a>
{{if .Drift}}<a href="#" data-once="drift">Drift</a>{{end}}
<a href="#" data-once="reload">Reload</a>
<a href="#" data-once="hide">Hide</a>
</nav>
</div>
<style>
#once-bar{position:fixed;top:0;right:0;left:0;z-index:2147483647;display:flex;align-items:center;gap:1.25rem;height:40px;padding:0 1rem;box-sizing:border-box;background:rgba(17,17,19,.72);border-bottom:1px solid rgba(255,255,255,.14);color:#f3ede1;font:500 11px/40px ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;letter-spacing:.12em;text-transform:uppercase;-webkit-backdrop-filter:blur(14px) saturate(140%);backdrop-filter:blur(14px) saturate(140%)}
#once-bar .once-bar-name{margin-right:auto;overflow:hidden;font-weight:600;letter-spacing:.16em;white-space:nowrap;text-overflow:ellipsis}
#once-bar .once-bar-nav{display:flex;align-items:center;gap:1.15rem}
#once-bar a{color:inherit;text-decoration:none;opacity:.7}
#once-bar a:hover,#once-bar a:focus-visible{opacity:1}
#once-bar a:focus-visible{outline:1px solid rgba(255,255,255,.5);outline-offset:2px}
@media (prefers-reduced-transparency:reduce){#once-bar{background:rgba(17,17,19,.96);-webkit-backdrop-filter:none;backdrop-filter:none}}
</style>
<script>
(function () {
  const bar = document.getElementById("once-bar")
  if (!bar) return
  const drift = {{.Drift}}
  bar.addEventListener("click", (event) => {
    const action = event.target.closest("[data-once]")
    if (!action) return
    switch (action.getAttribute("data-once")) {
      case "reload":
        location.reload()
        break
      case "hide":
        bar.hidden = true
        break
      case "drift":
        if (drift.length > 0) location.assign(drift[Math.floor(Math.random() * drift.length)])
        break
      default:
        return
    }
    event.preventDefault()
  })
})()
</script>`
