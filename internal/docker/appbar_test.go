package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppBarLoadsScriptFromServedCopy(t *testing.T) {
	ns := namespaceWithApps(ApplicationSettings{Name: "only", Host: "only.localhost"})

	tag := ns.Application("only").appBar()

	assert.Equal(t, `<script src="/__once/bar.js" defer></script>`, tag)
}

func TestAppBarScriptCarriesNavigationData(t *testing.T) {
	ns := namespaceWithApps(
		ApplicationSettings{Name: "alpha", Host: "alpha.localhost"},
		ApplicationSettings{Name: "beta", Host: "beta.localhost"},
		ApplicationSettings{Name: "gamma", Host: "gamma.localhost"},
	)

	script := ns.Application("beta").appBarScript()

	assert.Contains(t, script, `"name":"beta"`)
	assert.Contains(t, script, `"back":"http://alpha.localhost"`)
	assert.Contains(t, script, `"next":"http://gamma.localhost"`)
	assert.Contains(t, script, `"contents":"/"`)
	assert.Contains(t, script, `"drift":["http://alpha.localhost","http://gamma.localhost"]`)
}

func TestAppBarScriptWrapsNavigation(t *testing.T) {
	ns := namespaceWithApps(
		ApplicationSettings{Name: "beta", Host: "beta.localhost"},
		ApplicationSettings{Name: "gamma", Host: "gamma.localhost"},
	)
	app := &Application{
		namespace: ns,
		Settings:  ApplicationSettings{Name: "alpha", Host: "alpha.localhost"},
	}

	script := app.appBarScript()

	assert.Contains(t, script, `"back":"http://gamma.localhost"`)
	assert.Contains(t, script, `"next":"http://beta.localhost"`)
}

func TestAppBarScriptWithoutSiblings(t *testing.T) {
	ns := namespaceWithApps(ApplicationSettings{Name: "only", Host: "only.localhost"})

	script := ns.Application("only").appBarScript()

	assert.NotContains(t, script, `"back":`)
	assert.NotContains(t, script, `"next":`)
	assert.Contains(t, script, `"contents":"/"`)
	assert.Contains(t, script, `"drift":[]`)
}

func TestAppBarScriptIgnoresStoppedApps(t *testing.T) {
	ns := namespaceWithApps(
		ApplicationSettings{Name: "alpha", Host: "alpha.localhost"},
		ApplicationSettings{Name: "beta", Host: "beta.localhost"},
		ApplicationSettings{Name: "gamma", Host: "gamma.localhost"},
	)
	ns.Application("alpha").Running = false
	ns.Application("gamma").Running = false

	script := ns.Application("beta").appBarScript()

	assert.NotContains(t, script, "alpha.localhost")
	assert.NotContains(t, script, "gamma.localhost")
	assert.NotContains(t, script, `"back":`)
	assert.Contains(t, script, `"drift":[]`)
}

func TestAppBarScriptBindsKeyboardShortcuts(t *testing.T) {
	ns := namespaceWithApps(
		ApplicationSettings{Name: "alpha", Host: "alpha.localhost"},
		ApplicationSettings{Name: "beta", Host: "beta.localhost"},
	)

	script := ns.Application("beta").appBarScript()

	assert.Contains(t, script, `{"/":"menu","[":"back","]":"next","c":"contents","h":"hide"}`)
	assert.Contains(t, script, `event.shiftKey ? "reload" : "drift"`)
	assert.Contains(t, script, `event.key === "Escape"`)
	assert.Contains(t, script, `event.target.closest?.("input, textarea, select, [contenteditable]")`)
}

func TestAppBarScriptShowsShortcutHints(t *testing.T) {
	ns := namespaceWithApps(
		ApplicationSettings{Name: "alpha", Host: "alpha.localhost"},
		ApplicationSettings{Name: "beta", Host: "beta.localhost"},
	)

	script := ns.Application("beta").appBarScript()

	assert.Contains(t, script, `["back", "Back", "[", data.back]`)
	assert.Contains(t, script, `["next", "Next", "]", data.next]`)
	assert.Contains(t, script, `["contents", "Contents", "C", data.contents]`)
	assert.Contains(t, script, `["drift", "Drift", "R", "#"]`)
	assert.Contains(t, script, `["reload", "Reload", "shift R", "#"]`)
	assert.Contains(t, script, `["hide", "Hide", "H", "#"]`)
	assert.Contains(t, script, `toggle.setAttribute("aria-expanded"`)
}

func TestWriteAppBarScript(t *testing.T) {
	ns := namespaceWithApps(
		ApplicationSettings{Name: "alpha", Host: "alpha.localhost"},
		ApplicationSettings{Name: "beta", Host: "beta.localhost"},
	)
	served := t.TempDir()
	app := ns.Application("beta")
	app.Settings.ServedPath = served

	require.NoError(t, app.writeAppBarScript())

	written, err := os.ReadFile(filepath.Join(served, appBarScriptPath))
	require.NoError(t, err)
	assert.Equal(t, app.appBarScript(), string(written))
}

func TestWriteAppBarScriptWithoutServedCopy(t *testing.T) {
	app := &Application{Settings: ApplicationSettings{Name: "site", Source: "/srv/site"}}

	assert.NoError(t, app.writeAppBarScript())
}

func TestAppBarScriptCanBeServedAsIs(t *testing.T) {
	ns := namespaceWithApps(ApplicationSettings{Name: "alpha", Host: "alpha.localhost"})

	script := ns.Application("alpha").appBarScript()

	assert.NotContains(t, script, "{{")
	assert.Contains(t, script, `document.body.appendChild(bar)`)
	assert.Contains(t, script, "height:40px")
	assert.Contains(t, script, "backdrop-filter:blur")
}

func TestAppBarCrossFadesContentUnderStillBar(t *testing.T) {
	ns := namespaceWithApps(ApplicationSettings{Name: "alpha", Host: "alpha.localhost"})

	script := ns.Application("alpha").appBarScript()

	assert.Contains(t, script, `node !== bar && node !== tab && node.tagName !== "SCRIPT"`)
	assert.Contains(t, script, `node.style.transition = "opacity " + fadeDuration + "ms ease"`)
	assert.Contains(t, script, `window.setTimeout(() => location.assign(href), fadeDuration)`)
	assert.NotContains(t, script, "bar.style.opacity")
}

func TestAppBarCrossFadeFallsBackWithoutTransitions(t *testing.T) {
	ns := namespaceWithApps(ApplicationSettings{Name: "alpha", Host: "alpha.localhost"})

	script := ns.Application("alpha").appBarScript()

	assert.Contains(t, script, `(prefers-reduced-motion: reduce)`)
	assert.Contains(t, script, `const canFade = !reduceMotion && Boolean(window.CSS`)
	assert.Contains(t, script, `if (!canFade || nodes.length === 0) return location.assign(href)`)
}

func TestAppBarCrossFadeFadesInAfterAppChange(t *testing.T) {
	ns := namespaceWithApps(
		ApplicationSettings{Name: "alpha", Host: "alpha.localhost"},
		ApplicationSettings{Name: "beta", Host: "beta.localhost"},
	)

	script := ns.Application("beta").appBarScript()

	assert.Contains(t, script, `if (canFade && arrivedFromApp()) fadeIn(contentNodes())`)
	assert.Contains(t, script, `document.referrer.indexOf(url + "/") === 0`)
	assert.Contains(t, script, `if (event.persisted) contentNodes().forEach(clearFade)`)
}

func TestAppBarHidesOnScrollDownAndReturnsOnScrollUp(t *testing.T) {
	ns := namespaceWithApps(ApplicationSettings{Name: "alpha", Host: "alpha.localhost"})

	script := ns.Application("alpha").appBarScript()

	assert.Contains(t, script, `window.addEventListener("scroll", () => {`)
	assert.Contains(t, script, `if (scrollY <= 0) return setBarHidden(false)`)
	assert.Contains(t, script, `if (delta > 4) setBarHidden(true)`)
	assert.Contains(t, script, `else if (delta < -4) setBarHidden(false)`)
	assert.Contains(t, script, `{ passive: true }`)
}

func TestAppBarLeavesTabToRestoreWhenHidden(t *testing.T) {
	ns := namespaceWithApps(ApplicationSettings{Name: "alpha", Host: "alpha.localhost"})

	script := ns.Application("alpha").appBarScript()

	assert.Contains(t, script, `tab.id = "once-bar-tab"`)
	assert.Contains(t, script, `tab.hidden = true`)
	assert.Contains(t, script, `tab.hidden = !hidden`)
	assert.Contains(t, script, `tab.addEventListener("click"`)
	assert.Contains(t, script, `setBarHidden(false)`)
	assert.Contains(t, script, `#once-bar-tab{position:fixed;top:0;right:1rem`)
	assert.Contains(t, script, `#once-bar.once-bar-hidden{transform:translateY(-100%)}`)
}

func TestAppBarHideKeyTogglesInsteadOfSticking(t *testing.T) {
	ns := namespaceWithApps(ApplicationSettings{Name: "alpha", Host: "alpha.localhost"})

	script := ns.Application("alpha").appBarScript()

	assert.Contains(t, script, `{"/":"menu","[":"back","]":"next","c":"contents","h":"hide"}`)
	assert.Contains(t, script, `if (action === "hide") return setBarHidden(!barHidden)`)
	assert.NotContains(t, script, "bar.hidden = true")
}

func TestStaticNginxCommandInjectsAppBar(t *testing.T) {
	ns := namespaceWithApps(ApplicationSettings{Name: "alpha", Host: "alpha.localhost"})
	app := NewApplication(ns, ApplicationSettings{
		Name:   "site",
		Host:   "site.localhost",
		Source: "/srv/site",
	})

	command := strings.Join(app.staticNginxCommand(), " ")

	assert.Contains(t, command, `sub_filter '</body>' '<script src="/__once/bar.js" defer></script></body>'`)
	assert.Contains(t, command, "sub_filter_types text/html")
	assert.Contains(t, command, "location = /__once/bar.js")
}

func TestEscapeNginxVariables(t *testing.T) {
	assert.Equal(t, `cost\$site`, escapeNginxVariables("cost$site"))
}

func namespaceWithApps(settings ...ApplicationSettings) *Namespace {
	ns := &Namespace{proxy: &Proxy{}}
	for _, s := range settings {
		app := NewApplication(ns, s)
		app.Running = true
		ns.applications = append(ns.applications, app)
	}
	ns.sortApplications()
	return ns
}
