package docker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppBar(t *testing.T) {
	ns := namespaceWithApps(
		ApplicationSettings{Name: "alpha", Host: "alpha.localhost"},
		ApplicationSettings{Name: "beta", Host: "beta.localhost"},
		ApplicationSettings{Name: "gamma", Host: "gamma.localhost"},
	)
	app := ns.Application("beta")

	bar := app.appBar()

	assert.Contains(t, bar, `<span class="once-bar-name">beta</span>`)
	assert.Contains(t, bar, `href="http://alpha.localhost" rel="prev">Back</a>`)
	assert.Contains(t, bar, `href="http://gamma.localhost" rel="next">Next</a>`)
	assert.Contains(t, bar, `href="/">Contents</a>`)
	assert.Contains(t, bar, ">Drift</a>")
	assert.Contains(t, bar, ">Reload</a>")
	assert.Contains(t, bar, ">Hide</a>")
	assert.Contains(t, bar, "height:40px")
	assert.Contains(t, bar, "backdrop-filter:blur")
}

func TestAppBarWrapsNavigation(t *testing.T) {
	ns := namespaceWithApps(
		ApplicationSettings{Name: "beta", Host: "beta.localhost"},
		ApplicationSettings{Name: "gamma", Host: "gamma.localhost"},
	)
	app := &Application{
		namespace: ns,
		Settings:  ApplicationSettings{Name: "alpha", Host: "alpha.localhost"},
	}

	bar := app.appBar()

	assert.Contains(t, bar, `href="http://gamma.localhost" rel="prev">Back</a>`)
	assert.Contains(t, bar, `href="http://beta.localhost" rel="next">Next</a>`)
}

func TestAppBarWithoutSiblings(t *testing.T) {
	ns := namespaceWithApps(ApplicationSettings{Name: "only", Host: "only.localhost"})

	bar := ns.Application("only").appBar()

	assert.NotContains(t, bar, ">Back</a>")
	assert.NotContains(t, bar, ">Next</a>")
	assert.NotContains(t, bar, ">Drift</a>")
	assert.Contains(t, bar, ">Contents</a>")
}

func TestAppBarIgnoresStoppedApps(t *testing.T) {
	ns := namespaceWithApps(
		ApplicationSettings{Name: "alpha", Host: "alpha.localhost"},
		ApplicationSettings{Name: "beta", Host: "beta.localhost"},
		ApplicationSettings{Name: "gamma", Host: "gamma.localhost"},
	)
	ns.Application("alpha").Running = false
	ns.Application("gamma").Running = false

	bar := ns.Application("beta").appBar()

	assert.NotContains(t, bar, "alpha.localhost")
	assert.NotContains(t, bar, "gamma.localhost")
	assert.NotContains(t, bar, ">Back</a>")
	assert.NotContains(t, bar, ">Drift</a>")
}

func TestAppBarDriftOffersEveryOtherApp(t *testing.T) {
	ns := namespaceWithApps(
		ApplicationSettings{Name: "alpha", Host: "alpha.localhost"},
		ApplicationSettings{Name: "beta", Host: "beta.localhost"},
		ApplicationSettings{Name: "gamma", Host: "gamma.localhost"},
	)

	bar := ns.Application("beta").appBar()

	assert.Contains(t, bar, `const drift = ["http://alpha.localhost","http://gamma.localhost"]`)
}

func TestAppBarCanBeEmbeddedInNginxConfig(t *testing.T) {
	ns := namespaceWithApps(
		ApplicationSettings{Name: "alpha", Host: "alpha.localhost"},
		ApplicationSettings{Name: "beta", Host: "beta.localhost"},
	)

	bar := ns.Application("alpha").appBar()

	assert.NotContains(t, bar, "'")
	assert.NotContains(t, bar, "{{")
}

func TestStaticNginxCommandInjectsAppBar(t *testing.T) {
	ns := namespaceWithApps(ApplicationSettings{Name: "alpha", Host: "alpha.localhost"})
	app := NewApplication(ns, ApplicationSettings{
		Name:   "site",
		Host:   "site.localhost",
		Source: "/srv/site",
	})

	command := strings.Join(app.staticNginxCommand(), " ")

	assert.Contains(t, command, `sub_filter '</body>' '<div id="once-bar">`)
	assert.Contains(t, command, `<span class="once-bar-name">site</span>`)
	assert.Contains(t, command, "sub_filter_types text/html")
}

func TestStaticNginxCommandEscapesDollarSigns(t *testing.T) {
	app := &Application{Settings: ApplicationSettings{Name: "cost$site", Source: "/srv/site"}}

	command := strings.Join(app.staticNginxCommand(), " ")

	require.Contains(t, command, `cost\$site`)
	assert.NotContains(t, command, "cost$site")
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
