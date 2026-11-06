package docker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const appBarScriptPath = "__once/bar.js"

// appBarData is everything the bar over a served page needs to know.
type appBarData struct {
	Name     string   `json:"name"`
	Back     string   `json:"back,omitempty"`
	Next     string   `json:"next,omitempty"`
	Contents string   `json:"contents"`
	Drift    []string `json:"drift"`
}

// Private

// appBar returns the tag that loads the bar for the served page. The bar
// itself lives in a file inside the site copy, so its size is not limited by
// the nginx config parser.
func (a *Application) appBar() string {
	return `<script src="/` + appBarScriptPath + `" defer></script>`
}

// writeAppBarScript writes the bar next to the served pages. The source folder
// is never touched.
func (a *Application) writeAppBarScript() error {
	if a.Settings.ServedPath == "" {
		return nil
	}

	path := filepath.Join(a.Settings.ServedPath, filepath.FromSlash(appBarScriptPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(a.appBarScript()), 0o644)
}

func (a *Application) appBarScript() string {
	data, err := json.Marshal(a.appBarData())
	if err != nil {
		return ""
	}
	return strings.Replace(appBarScriptTemplate, "{{data}}", string(data), 1)
}

func (a *Application) appBarData() appBarData {
	data := appBarData{
		Name:     a.Settings.Name,
		Contents: "/",
		Drift:    []string{},
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

const appBarScriptTemplate = `(function () {
  const data = {{data}}
  const drift = data.drift || []
  const links = {}

  const bar = document.createElement("div")
  bar.id = "once-bar"

  const name = document.createElement("span")
  name.className = "once-bar-name"
  name.textContent = data.name
  bar.appendChild(name)

  const nav = document.createElement("nav")
  nav.className = "once-bar-nav"
  bar.appendChild(nav)

  const menu = document.createElement("div")
  menu.id = "once-bar-menu"
  menu.hidden = true
  bar.appendChild(menu)

  const actions = []
  if (data.back) actions.push(["back", "Back", "[", data.back])
  if (data.next) actions.push(["next", "Next", "]", data.next])
  actions.push(["contents", "Contents", "C", data.contents])
  if (drift.length > 0) actions.push(["drift", "Drift", "R", "#"])
  actions.push(["reload", "Reload", "shift R", "#"], ["hide", "Hide", "H", "#"])

  actions.forEach(([action, label, key, href]) => {
    for (const target of [nav, menu]) {
      const link = document.createElement("a")
      link.textContent = label
      link.href = href
      link.dataset.once = action
      if (target === nav) links[action] = link
      if (target === menu) {
        const hint = document.createElement("kbd")
        hint.textContent = key
        link.appendChild(hint)
      }
      target.appendChild(link)
    }
  })

  const toggle = document.createElement("a")
  toggle.textContent = "Menu"
  toggle.href = "#"
  toggle.dataset.once = "menu"
  toggle.setAttribute("aria-haspopup", "true")
  toggle.setAttribute("aria-expanded", "false")
  nav.appendChild(toggle)

  const style = document.createElement("style")
  style.textContent = ` + "`" + `
#once-bar{position:fixed;top:0;right:0;left:0;z-index:2147483647;display:flex;align-items:center;gap:1.25rem;height:40px;padding:0 1rem;box-sizing:border-box;background:rgba(17,17,19,.72);border-bottom:1px solid rgba(255,255,255,.14);color:#f3ede1;font:500 11px/40px ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;letter-spacing:.12em;text-transform:uppercase;-webkit-backdrop-filter:blur(14px) saturate(140%);backdrop-filter:blur(14px) saturate(140%)}
#once-bar .once-bar-name{margin-right:auto;overflow:hidden;font-weight:600;letter-spacing:.16em;white-space:nowrap;text-overflow:ellipsis}
#once-bar .once-bar-nav{display:flex;align-items:center;gap:1.15rem}
#once-bar a{color:inherit;text-decoration:none;opacity:.7}
#once-bar a:hover,#once-bar a:focus-visible{opacity:1}
#once-bar a:focus-visible{outline:1px solid rgba(255,255,255,.5);outline-offset:2px}
#once-bar-menu{position:absolute;top:calc(100% + 6px);right:1rem;display:grid;gap:.1rem;min-width:13rem;padding:.35rem;background:rgba(17,17,19,.9);border:1px solid rgba(255,255,255,.14);border-radius:6px;box-shadow:0 12px 32px rgba(0,0,0,.35);-webkit-backdrop-filter:blur(14px) saturate(140%);backdrop-filter:blur(14px) saturate(140%)}
#once-bar-menu[hidden]{display:none}
#once-bar-menu a{display:flex;align-items:center;justify-content:space-between;gap:2rem;padding:.45rem .6rem;font:inherit;letter-spacing:.12em;opacity:.8}
#once-bar-menu a:hover,#once-bar-menu a:focus-visible{background:rgba(255,255,255,.08);opacity:1}
#once-bar-menu kbd{font:inherit;letter-spacing:.08em;opacity:.55}
@media (prefers-reduced-transparency:reduce){#once-bar,#once-bar-menu{background:rgba(17,17,19,.96);-webkit-backdrop-filter:none;backdrop-filter:none}}
` + "`" + `

  function setMenu(open) {
    menu.hidden = !open
    toggle.setAttribute("aria-expanded", open ? "true" : "false")
  }

  const reduceMotion = Boolean(window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches)
  const canFade = !reduceMotion && Boolean(window.CSS && window.CSS.supports && window.CSS.supports("transition", "opacity"))
  const fadeDuration = 180

  function contentNodes() {
    return Array.from(document.body.children).filter((node) => node !== bar && node.tagName !== "SCRIPT")
  }

  function clearFade(node) {
    node.style.transition = ""
    node.style.opacity = ""
  }

  function fadeIn(nodes) {
    nodes.forEach((node) => {
      node.style.transition = "none"
      node.style.opacity = "0"
    })
    void document.body.offsetWidth
    nodes.forEach((node) => {
      node.style.transition = "opacity " + fadeDuration + "ms ease"
      node.style.opacity = "1"
    })
    window.setTimeout(() => nodes.forEach(clearFade), fadeDuration + 50)
  }

  function arrivedFromApp() {
    if (!document.referrer) return false
    const known = [data.back, data.next].concat(drift).filter((url) => url && url !== "#")
    return known.some((url) => document.referrer === url || document.referrer.indexOf(url + "/") === 0)
  }

  function leave(href) {
    if (!href || href === "#") return
    const nodes = contentNodes()
    if (!canFade || nodes.length === 0) return location.assign(href)
    nodes.forEach((node) => {
      node.style.transition = "opacity " + fadeDuration + "ms ease"
      node.style.opacity = "0"
    })
    window.setTimeout(() => location.assign(href), fadeDuration)
  }

  function run(action) {
    if (action === "menu") return setMenu(menu.hidden)
    if (action === "hide") { setMenu(false); bar.hidden = true; return }
    if (action === "reload") return location.reload()
    if (action === "drift") {
      if (drift.length > 0) leave(drift[Math.floor(Math.random() * drift.length)])
      return
    }
    if (links[action]) leave(links[action].href)
  }

  bar.addEventListener("click", (event) => {
    const action = event.target.closest?.("[data-once]")
    if (!action) return
    run(action.dataset.once)
    event.preventDefault()
  })

  document.addEventListener("click", (event) => {
    if (!event.target.closest?.("#once-bar")) setMenu(false)
  })

  document.addEventListener("keydown", (event) => {
    if (event.ctrlKey || event.metaKey || event.altKey) return
    if (event.target.closest?.("input, textarea, select, [contenteditable]")) return
    if (event.key === "Escape") return setMenu(false)
    if (event.key.toLowerCase() === "r") {
      event.preventDefault()
      return run(event.shiftKey ? "reload" : "drift")
    }
    const bindings = {"/":"menu","[":"back","]":"next","c":"contents","h":"hide"}
    const action = bindings[event.key.toLowerCase()]
    if (!action) return
    event.preventDefault()
    run(action)
  })

  document.head.appendChild(style)
  document.body.appendChild(bar)

  if (canFade && arrivedFromApp()) fadeIn(contentNodes())

  window.addEventListener("pageshow", (event) => {
    if (event.persisted) contentNodes().forEach(clearFade)
  })
})()
`
