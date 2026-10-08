package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type colorTheme struct {
	ID, Name, Primary, Secondary, Muted, Text, Border, Fill, Background string
}

var themeCatalog = []colorTheme{
	{"hypex", "Hypex", "#52E0D0", "#C792EA", "#8792A8", "#E8EDF7", "#52617A", "#248F91", "#1A1B26"},
	{"github-dark", "GitHub Dark", "#58A6FF", "#3FB950", "#8B949E", "#E6EDF3", "#30363D", "#238636", "#0D1117"},
	{"github-light", "GitHub Light", "#0969DA", "#1A7F37", "#656D76", "#24292F", "#D0D7DE", "#D8DEE4", "#FFFFFF"},
	{"tokyo-night", "Tokyo Night", "#7AA2F7", "#BB9AF7", "#565F89", "#C0CAF5", "#3B4261", "#414868", "#1A1B26"},
	{"one-dark", "One Dark", "#61AFEF", "#C678DD", "#7F848E", "#ABB2BF", "#3E4451", "#4B5263", "#282C34"},
	{"vercel", "Vercel", "#EDEDED", "#0070F3", "#888888", "#FAFAFA", "#333333", "#222222", "#000000"},
	{"catppuccin-mocha", "Catppuccin Mocha", "#89B4FA", "#CBA6F7", "#A6ADC8", "#CDD6F4", "#45475A", "#313244", "#1E1E2E"},
	{"catppuccin-latte", "Catppuccin Latte", "#1E66F5", "#8839EF", "#6C6F85", "#4C4F69", "#BCC0CC", "#DCE0E8", "#EFF1F5"},
	{"dracula", "Dracula", "#8BE9FD", "#BD93F9", "#6272A4", "#F8F8F2", "#44475A", "#343746", "#282A36"},
	{"nord", "Nord", "#88C0D0", "#81A1C1", "#7B88A1", "#D8DEE9", "#434C5E", "#3B4252", "#2E3440"},
	{"gruvbox", "Gruvbox", "#FABD2F", "#FE8019", "#A89984", "#EBDBB2", "#504945", "#3C3836", "#282828"},
	{"solarized-dark", "Solarized Dark", "#268BD2", "#2AA198", "#839496", "#EEE8D5", "#586E75", "#073642", "#002B36"},
}

var iconPalette = []color.Color{cyan, purple, lipgloss.Color("#7AA2F7"), lipgloss.Color("#2AC3DE"), lipgloss.Color("#F7768E"), lipgloss.Color("#9ECE6A")}
var activeTheme = themeCatalog[0]
var themeBackground = lipgloss.Color(activeTheme.Background)
var themeFill = lipgloss.Color(activeTheme.Fill)
var activeFontStyle = "classic"

type titleFont struct{ ID, Name, Hypex, Fetch string }

var fontCatalog = []titleFont{
	{"classic", "Classic", "Hypex", "Fetch"},
	{"bold", "Bold Sans", "𝗛𝘆𝗽𝗲𝘅", "𝗙𝗲𝘁𝗰𝗵"},
	{"mono", "Monospace", "𝙷𝚢𝚙𝚎𝚡", "𝙵𝚎𝚝𝚌𝚑"},
	{"double", "Double Struck", "ℍ𝕪𝕡𝕖𝕩", "𝔽𝕖𝕥𝕔𝕙"},
	{"small-caps", "Small Caps", "Hʏᴘᴇx", "Fᴇᴛᴄʜ"},
}

func selectedTitleFont(id string) titleFont {
	for _, font := range fontCatalog {
		if font.ID == id {
			return font
		}
	}
	return fontCatalog[0]
}

type themeStore struct {
	Version   int    `json:"version"`
	Selected  string `json:"selected_theme"`
	FontStyle string `json:"title_font"`
}

func applyTheme(name string) error {
	for _, theme := range themeCatalog {
		if strings.EqualFold(theme.ID, name) || strings.EqualFold(theme.Name, name) {
			activeTheme = theme
			cyan, purple = lipgloss.Color(theme.Primary), lipgloss.Color(theme.Secondary)
			muted, text, border = lipgloss.Color(theme.Muted), lipgloss.Color(theme.Text), lipgloss.Color(theme.Border)
			panelBG, themeBackground, themeFill = lipgloss.Color(theme.Background), lipgloss.Color(theme.Background), lipgloss.Color(theme.Fill)
			iconPalette = []color.Color{cyan, purple, lipgloss.Color(theme.Primary), lipgloss.Color(theme.Secondary), lipgloss.Color(theme.Muted), lipgloss.Color(theme.Text)}
			return nil
		}
	}
	return fmt.Errorf("unknown theme %q (use --list-themes to see choices)", name)
}

func loadThemePreferences() themeStore {
	path, err := themeStorePath()
	if err != nil {
		return themeStore{Selected: "hypex", FontStyle: "classic"}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return themeStore{Selected: "hypex", FontStyle: "classic"}
	}
	store := themeStore{Selected: "hypex", FontStyle: "classic"}
	if json.Unmarshal(data, &store) != nil {
		return themeStore{Selected: "hypex", FontStyle: "classic"}
	}
	validTheme, validFont := false, false
	for _, theme := range themeCatalog {
		if theme.ID == store.Selected {
			validTheme = true
			break
		}
	}
	for _, font := range fontCatalog {
		if font.ID == store.FontStyle {
			validFont = true
			break
		}
	}
	if !validTheme {
		store.Selected = "hypex"
	}
	if !validFont {
		store.FontStyle = "classic"
	}
	return store
}

func saveTheme(name, fontStyle string) error {
	path, err := themeStorePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(themeStore{Version: 1, Selected: name, FontStyle: fontStyle}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

func themeStorePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "hypexfetch", "themes.json"), nil
}

type themeEditorModel struct {
	width, height int
	selectedTheme int
	selectedFont  int
	section       int
	frame         int
	status        string
}

type themeEditorTick struct{}

func (m themeEditorModel) Init() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg { return themeEditorTick{} })
}

func (m themeEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case themeEditorTick:
		m.frame++
		return m, tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg { return themeEditorTick{} })
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.section == 0 {
				m.selectedTheme = (m.selectedTheme - 1 + len(themeCatalog)) % len(themeCatalog)
			} else {
				m.selectedFont = (m.selectedFont - 1 + len(fontCatalog)) % len(fontCatalog)
			}
			m.status = ""
		case "down", "j":
			if m.section == 0 {
				m.selectedTheme = (m.selectedTheme + 1) % len(themeCatalog)
			} else {
				m.selectedFont = (m.selectedFont + 1) % len(fontCatalog)
			}
			m.status = ""
		case "tab", "left", "right":
			m.section = 1 - m.section
		case "enter", "s":
			if err := saveTheme(themeCatalog[m.selectedTheme].ID, fontCatalog[m.selectedFont].ID); err != nil {
				m.status = "Couldn't save: " + err.Error()
				return m, nil
			}
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m themeEditorModel) View() tea.View {
	theme := themeCatalog[m.selectedTheme]
	previewFont := fontCatalog[m.selectedFont]
	menuPrimary := lipgloss.Color("#52E0D0")
	menuSecondary := lipgloss.Color("#C792EA")
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8792A8"))
	selectedStyle := lipgloss.NewStyle().Foreground(menuPrimary).Bold(true)
	items := make([]string, 0, len(themeCatalog))
	if m.section == 0 {
		for i, item := range themeCatalog {
			marker := "  "
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EDF7"))
			if i == m.selectedTheme {
				marker = "▸ "
				style = selectedStyle
			}
			items = append(items, style.Render(marker+item.Name))
		}
	} else {
		for i, item := range fontCatalog {
			marker := "  "
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EDF7"))
			if i == m.selectedFont {
				marker = "▸ "
				style = selectedStyle
			}
			items = append(items, style.Render(marker+item.Name))
		}
	}

	previewWidth := max(24, min(46, m.width-34))
	preview := renderThemePreview(theme, previewWidth, m.frame, previewFont)
	leftWidth := max(20, m.width-previewWidth-8)
	paletteTab := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EDF7")).Render("Palettes")
	fontTab := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EDF7")).Render("Title font")
	if m.section == 0 {
		paletteTab = lipgloss.NewStyle().Foreground(menuPrimary).Bold(true).Render("▸ Palettes")
	} else {
		fontTab = lipgloss.NewStyle().Foreground(menuPrimary).Bold(true).Render("▸ Title font")
	}
	selectorTitle := lipgloss.JoinHorizontal(lipgloss.Top, paletteTab, "    ", fontTab)
	list := lipgloss.NewStyle().Width(leftWidth).Render(lipgloss.JoinVertical(lipgloss.Left, selectorTitle, "", strings.Join(items, "\n")))
	content := ""
	if m.width >= 74 {
		content = lipgloss.JoinHorizontal(lipgloss.Top, list, strings.Repeat(" ", 3), preview)
	} else {
		content = list + "\n\n" + preview
	}

	title := lipgloss.NewStyle().Foreground(menuPrimary).Bold(true).Render("HypexFetch Customizer")
	subtitle := mutedStyle.Render("Preview palette and title font before applying")
	help := lipgloss.NewStyle().Foreground(menuSecondary).Render("Tab switch   ↑/↓ choose   Enter apply & save   q cancel")
	footer := help
	if m.status != "" {
		footer = mutedStyle.Render(m.status)
	}
	body := lipgloss.JoinVertical(lipgloss.Left, title, subtitle, "", content, "", footer)
	body = lipgloss.NewStyle().Padding(1, 2).Render(body)
	view := tea.NewView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body))
	view.AltScreen = true
	view.BackgroundColor = lipgloss.Color("#1A1B26")
	return view
}

func renderThemePreview(theme colorTheme, width, frame int, font titleFont) string {
	primary := lipgloss.Color(theme.Primary)
	secondary := lipgloss.Color(theme.Secondary)
	bg := lipgloss.Color(theme.Background)
	line := lipgloss.NewStyle().Foreground(primary).Bold(true)
	insideWidth := max(16, width-2)
	barColors := []string{theme.Primary, theme.Secondary, theme.Fill, theme.Border}
	var bar strings.Builder
	for i := 0; i < insideWidth; i++ {
		idx := (i/3 + frame/2) % len(barColors)
		bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(barColors[idx])).Render("━"))
	}
	rows := []string{
		line.Render(font.Hypex) + lipgloss.NewStyle().Foreground(secondary).Bold(true).Render(font.Fetch),
		lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Muted)).Render("user@device"),
		bar.String(),
		lipgloss.NewStyle().Foreground(secondary).Bold(true).Render("OS      Android"),
		lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Text)).Render("Kernel  6.1.0"),
		lipgloss.NewStyle().Foreground(primary).Render("Memory  4.2 / 8.0 GB"),
	}
	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(width).Padding(1).Border(lipgloss.RoundedBorder()).BorderForeground(primary).Background(bg).Render(content)
}
