package ui

import (
	"image/color"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/basecamp/once/internal/docker"
)

const (
	contentsCardMaxWidth = 64
	contentsCardPadding  = 2
)

var contentsCardCloseKey = WithHelp(NewKeyBinding("esc"), "esc", "back")
var contentsCardFavoriteKey = WithHelp(NewKeyBinding("f"), "f", "favorite")

var (
	contentsPaper  = lipgloss.Color("#f4ecdc")
	contentsInk    = lipgloss.Color("#241f1a")
	contentsRule   = lipgloss.Color("#d8cbb4")
	contentsMuted  = lipgloss.Color("#7b7166")
	contentsAccent = lipgloss.Color("#b4552f")
)

type ContentsCardCloseMsg struct{}

type ContentsCardFavoriteMsg struct {
	AppName  string
	Favorite bool
}

type ContentsCard struct {
	app        docker.Application
	lastBackup *docker.OperationResult
	favorite   bool
	width      int
	height     int
}

func NewContentsCard(app *docker.Application, lastBackup *docker.OperationResult, favorite bool, width int) ContentsCard {
	return ContentsCard{
		app:        *app,
		lastBackup: lastBackup,
		favorite:   favorite,
		width:      width,
	}
}

func (c ContentsCard) Init() tea.Cmd {
	return nil
}

func (c ContentsCard) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width, c.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		if key.Matches(msg, contentsCardFavoriteKey) {
			c.favorite = !c.favorite
			return c, func() tea.Msg {
				return ContentsCardFavoriteMsg{AppName: c.app.Settings.Name, Favorite: c.favorite}
			}
		}
		if key.Matches(msg, contentsCardCloseKey) {
			return c, func() tea.Msg { return ContentsCardCloseMsg{} }
		}
	}

	return c, nil
}

func (c ContentsCard) View() string {
	inner := c.contentWidth()

	lines := c.headerLines(inner)
	lines = append(lines, "")
	lines = append(lines, contentsCell(strings.Repeat("─", inner), inner, contentsStyle(contentsRule)))
	lines = append(lines, "")
	lines = append(lines, c.factLines(inner)...)
	lines = append(lines, "")
	lines = append(lines, contentsCell(c.footerText(), inner, contentsStyle(contentsMuted)))

	card := c.frame(lines, inner)

	return lipgloss.Place(c.width, c.height, lipgloss.Center, lipgloss.Center, card)
}

// Private

type contentsFact struct {
	label  string
	value  string
	accent bool
	muted  bool
}

func (c ContentsCard) headerLines(inner int) []string {
	stamp := c.stamp()
	titleWidth := max(inner-lipgloss.Width(stamp)-2, 12)

	left := []string{
		contentsCell("ONCE · CONTENTS", titleWidth, contentsStyle(contentsMuted)),
		contentsCell("", titleWidth, contentsStyle(contentsPaper)),
		c.titleLine(titleWidth),
	}

	stampLines := strings.Split(stamp, "\n")
	lines := make([]string, max(len(left), len(stampLines)))
	for i := range lines {
		var leftLine, stampLine string
		if i < len(left) {
			leftLine = left[i]
		} else {
			leftLine = contentsCell("", titleWidth, contentsStyle(contentsPaper))
		}
		if i < len(stampLines) {
			stampLine = stampLines[i]
		}
		lines[i] = leftLine + contentsCell("", 2, contentsStyle(contentsPaper)) + stampLine
	}

	return lines
}

func (c ContentsCard) titleLine(width int) string {
	starColor := contentsMuted
	if c.favorite {
		starColor = contentsAccent
	}
	star := contentsStyle(starColor).Render(favoriteStarText(c.favorite))
	return star + contentsCell(" "+c.displayName(), width-1, contentsStyle(contentsInk).Bold(true).Italic(true))
}

func (c ContentsCard) footerText() string {
	if c.favorite {
		return "f  unfavorite    esc  close"
	}
	return "f  favorite    esc  close"
}

func (c ContentsCard) stamp() string {
	initials := c.initials()
	center := func(s string) string {
		gap := max(4-lipgloss.Width(s), 0)
		left := gap / 2
		return strings.Repeat(" ", left) + s + strings.Repeat(" ", gap-left)
	}

	style := contentsStyle(contentsAccent).Bold(true)
	return strings.Join([]string{
		style.Render("╔════╗"),
		style.Render("║" + center(initials) + "║"),
		style.Render("╚════╝"),
	}, "\n")
}

func (c ContentsCard) factLines(inner int) []string {
	const labelWidth = 13
	valueWidth := max(inner-labelWidth, 8)

	facts := []contentsFact{
		{label: "HOST", value: c.host()},
		c.sourceFact(),
		c.uptimeFact(),
		{label: "LAST DEPLOY", value: formatContentsTime(c.app.DeployedAt)},
		c.backupFact(),
	}

	lines := make([]string, len(facts))
	for i, fact := range facts {
		valueStyle := contentsStyle(contentsInk)
		switch {
		case fact.accent:
			valueStyle = contentsStyle(contentsAccent)
		case fact.muted:
			valueStyle = contentsStyle(contentsMuted)
		}

		lines[i] = contentsCell(fact.label, labelWidth, contentsStyle(contentsMuted)) +
			contentsCell(fact.value, valueWidth, valueStyle)
	}

	return lines
}

func (c ContentsCard) host() string {
	if c.app.Settings.Host == "" {
		return "—"
	}
	return c.app.Settings.Host
}

func (c ContentsCard) sourceFact() contentsFact {
	if c.app.Settings.Source != "" {
		return contentsFact{label: "FOLDER", value: c.app.Settings.Source}
	}

	return contentsFact{label: "IMAGE", value: c.app.Settings.Image}
}

func (c ContentsCard) uptimeFact() contentsFact {
	if !c.app.Running {
		return contentsFact{label: "UPTIME", value: "stopped", muted: true}
	}
	if c.app.RunningSince.IsZero() {
		return contentsFact{label: "UPTIME", value: "running"}
	}

	return contentsFact{label: "UPTIME", value: formatDuration(time.Since(c.app.RunningSince))}
}

func (c ContentsCard) backupFact() contentsFact {
	if c.lastBackup == nil || c.lastBackup.At.IsZero() {
		return contentsFact{label: "LAST BACKUP", value: "Never", muted: true}
	}

	fact := contentsFact{label: "LAST BACKUP", value: formatContentsTime(c.lastBackup.At)}
	if c.lastBackup.Error != "" {
		fact.value += " · failed"
		fact.accent = true
	}

	return fact
}

func (c ContentsCard) displayName() string {
	words := c.nameWords()
	if len(words) == 0 {
		return "App"
	}

	for i, word := range words {
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}

	return strings.Join(words, " ")
}

func (c ContentsCard) initials() string {
	words := c.nameWords()
	if len(words) == 0 {
		return "?"
	}

	if len(words) > 1 {
		return strings.ToUpper(firstRune(words[0]) + firstRune(words[1]))
	}

	runes := []rune(words[0])
	if len(runes) == 1 {
		return strings.ToUpper(string(runes))
	}

	return strings.ToUpper(string(runes[:2]))
}

func (c ContentsCard) nameWords() []string {
	name := strings.NewReplacer("-", " ", "_", " ", ".", " ").Replace(c.app.Settings.Name)
	return strings.Fields(name)
}

func (c ContentsCard) contentWidth() int {
	width := min(contentsCardMaxWidth, c.width-2)
	return max(width-2*contentsCardPadding, 24)
}

func (c ContentsCard) frame(lines []string, inner int) string {
	border := contentsStyle(contentsRule)
	paper := contentsStyle(contentsPaper)
	padding := paper.Render(strings.Repeat(" ", contentsCardPadding))

	framed := make([]string, 0, len(lines)+2)
	framed = append(framed, border.Render("╭"+strings.Repeat("─", inner+2*contentsCardPadding)+"╮"))
	for _, line := range lines {
		if gap := inner - ansi.StringWidth(line); gap > 0 {
			line += paper.Render(strings.Repeat(" ", gap))
		}
		framed = append(framed, border.Render("│")+padding+line+padding+border.Render("│"))
	}
	framed = append(framed, border.Render("╰"+strings.Repeat("─", inner+2*contentsCardPadding)+"╯"))

	return strings.Join(framed, "\n")
}

// Helpers

func contentsStyle(fg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(fg).Background(contentsPaper)
}

func contentsCell(s string, width int, style lipgloss.Style) string {
	s = ansi.Truncate(s, width, "…")
	if gap := width - ansi.StringWidth(s); gap > 0 {
		s += strings.Repeat(" ", gap)
	}

	return style.Render(s)
}

func formatContentsTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("2 Jan 2006, 15:04")
}

func firstRune(s string) string {
	for _, r := range s {
		return string(r)
	}
	return ""
}
