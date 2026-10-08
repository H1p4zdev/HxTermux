package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type page int

const (
	welcome page = iota
	componentsPage
	customizePage
	reviewPage
	installingPage
	finishedPage
	restoreListPage
	restoreConfirmPage
)

type component struct {
	name, desc string
	selected   bool
}
type model struct {
	page                                                    page
	width, height, cursor, theme, font, prompt, promptField int
	components                                              []component
	status                                                  string
	logs                                                    []string
	err                                                     error
	started                                                 bool
	customizeOnly                                           bool
	editingHost                                             bool
	fontPreviewed                                           bool
	host                                                    string
	initialFont                                             int
	spinner                                                 spinner.Model
	backups                                                 []backupInfo
	backupCursor                                            int
	operation                                               string
}
type installMsg struct {
	lines []string
	err   error
}
type statusMsg string
type fontPreviewMsg struct {
	err  error
	quit bool
}
type backupInfo struct {
	ID, Kind    string
	Created     time.Time
	Entries     int
	Complete    bool
	SourcePath  string
	LegacyScope string
	LegacyRel   string
}

var themes = []string{"ayu-dark", "ayu-light", "ayu-mirage", "catppuccin", "dracula", "elementary", "everblush", "flat", "gruvbox-dark", "material-ocean", "material", "monokai-dark", "nekonako-djancoeg", "nekonako-hue", "nekonako-om-mar", "one-dark", "owl4ce-dark", "owl4ce-light", "siduck-onedark", "snazzy", "tomorrow-night", "tomorrow-night.eighties", "xshin"}
var colors = []string{"#79c0ff", "#6c99bb", "#ffcc66", "#cba6f7", "#ff7b72", "#a6e3a1", "#a6e3a1", "#fab387", "#fab387", "#89b4fa", "#89b4fa", "#f5c2e7", "#fd6b85", "#fb749f", "#f7c35f", "#e5c07b", "#f38ba8", "#f38ba8", "#c678dd", "#f38ba8", "#7dcfff", "#7dcfff", "#50fa7b"}
var promptThemes = []string{"ma", "ar-round", "archcraft", "la-round", "osx", "osx2", "rounded-custom", "rounded", "simple", "robbyrussell", "agnoster", "bira", "eastwood", "af-magic", "none"}

//go:embed .colorscheme/*.colors
var paletteFiles embed.FS

var activeProgram *tea.Program
var activeSnapshot string

func main() {
	setup := flag.Bool("setup", false, "run the installation wizard")
	customize := flag.Bool("customize", false, "open the appearance studio")
	restore := flag.Bool("restore", false, "restore a saved configuration snapshot")
	flag.Parse()
	startPage := customizePage
	if *setup {
		startPage = welcome
	}
	if *customize {
		startPage = customizePage
	}
	if *restore {
		startPage = restoreListPage
	}
	setupMarker := filepath.Join(os.Getenv("HOME"), ".config/hxtermux/setup-complete")
	_, setupErr := os.Stat(setupMarker)
	setupComplete := setupErr == nil
	if !*setup && !*restore && !setupComplete {
		startPage = welcome
	}
	theme, font, prompt, host := savedAppearance()
	m := model{page: startPage, theme: theme, font: font, prompt: prompt, host: host, initialFont: font, customizeOnly: !*setup && setupComplete, spinner: spinner.New(spinner.WithSpinner(spinner.Dot)), components: []component{
		{"Core shell tools", "git · curl · eza · fzf · lf · tmux · zsh · Termux:API", true},
		{"HxTermux dotfiles", "Shell, terminal settings, aliases and helper scripts", true},
		{"Zsh experience", "Oh My Zsh with autosuggestions and syntax highlighting", true},
		{"HypexFetch", "Your animated Bubble Tea system fetch", true},
		{"Awesomeshot", "Screenshot utility from its official Termux source branch", true},
		{"Neovim starter", "NvChad with Termux build tools and first-run plugin sync", true},
	}}
	if *restore {
		m.backups, _ = listBackups(os.Getenv("HOME"))
		m.operation = "restore"
	}
	p := tea.NewProgram(m)
	activeProgram = p
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "HxTermux:", err)
		os.Exit(1)
	}
}

func savedAppearance() (int, int, int, string) {
	themeIndex, fontIndex, promptIndex := 1, 0, 0
	host := ""
	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config/hxtermux/preferences"))
	if err != nil {
		return themeIndex, fontIndex, promptIndex, host
	}
	var themeName, fontName, promptName string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "theme=") {
			themeName = strings.TrimPrefix(line, "theme=")
		}
		if strings.HasPrefix(line, "font=") {
			fontName = strings.TrimPrefix(line, "font=")
		}
		if strings.HasPrefix(line, "prompt=") {
			promptName = strings.TrimPrefix(line, "prompt=")
		}
		if strings.HasPrefix(line, "host=") {
			host = strings.TrimPrefix(line, "host=")
		}
	}
	for i, theme := range themes {
		if theme == themeName {
			themeIndex = i
			break
		}
	}
	for i, font := range fontNames {
		if font == fontName {
			fontIndex = i
			break
		}
	}
	for i, prompt := range promptThemes {
		if prompt == promptName {
			promptIndex = i
			break
		}
	}
	if promptName == "" {
		if zshrc, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".zshrc")); err == nil {
			for _, line := range strings.Split(string(zshrc), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "ZSH_THEME=") {
					name := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "ZSH_THEME=")), "\"'")
					if name == "" {
						name = "none"
					}
					for i, prompt := range promptThemes {
						if name == prompt {
							promptIndex = i
						}
					}
				}
			}
		}
	}
	return themeIndex, fontIndex, promptIndex, host
}

var fontNames = []string{"Fira Code Bold Nerd Font.ttf", "Fira Code Medium Nerd Font Complete Mono.ttf", "JetBrains Mono Bold Nerd Font Complete.ttf", "JetBrains Mono Medium Nerd Font Complete.ttf", "MesloLGS NF Bold Italic.ttf", "MesloLGS NF Bold.ttf", "MesloLGS NF Italic.ttf", "MesloLGS NF Regular.ttf"}

func applyOnly(m model) tea.Cmd {
	return func() tea.Msg {
		err := applyAppearance("", os.Getenv("HOME"), m.theme, m.font, m.prompt, m.host)
		if err == nil {
			if _, statErr := os.Stat(filepath.Join(os.Getenv("HOME"), ".zshrc")); statErr == nil {
				err = applyPromptTheme(os.Getenv("HOME"), promptThemes[m.prompt%len(promptThemes)], m.host)
			}
		}
		if err != nil {
			return installMsg{err: err}
		}
		return installMsg{lines: []string{"Appearance saved"}}
	}
}

func validHostRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
}

func previewFont(index int) tea.Cmd {
	return func() tea.Msg {
		home := os.Getenv("HOME")
		font := fontNames[index%len(fontNames)]
		source := filepath.Join(home, ".local/share/hxtermux/fonts", font)
		if err := copyFile(source, filepath.Join(home, ".termux/font.ttf"), 0644); err != nil {
			return fontPreviewMsg{err: fmt.Errorf("font preview failed: %w", err)}
		}
		if command, err := exec.LookPath("termux-reload-settings"); err == nil {
			if err = exec.Command(command).Run(); err != nil {
				return fontPreviewMsg{err: fmt.Errorf("could not reload the font preview: %w", err)}
			}
		}
		return fontPreviewMsg{}
	}
}

func restoreFontPreview(index int) tea.Cmd {
	return func() tea.Msg {
		msg := previewFont(index)().(fontPreviewMsg)
		msg.quit = true
		return msg
	}
}

func (m model) Init() tea.Cmd { return m.spinner.Tick }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
	case installMsg:
		m.logs = v.lines
		m.err = v.err
		if v.err == nil {
			switch m.operation {
			case "setup":
				m.page = customizePage
				m.customizeOnly = true
				m.initialFont = m.font
				m.fontPreviewed = false
				m.status = ""
			case "exit":
				return m, tea.Quit
			default:
				m.page = finishedPage
			}
		} else {
			if m.operation == "exit" {
				return m, tea.Quit
			}
			m.status = v.err.Error()
		}
		return m, nil
	case fontPreviewMsg:
		if v.err != nil {
			m.status = v.err.Error()
		} else {
			m.status = ""
		}
		if v.quit {
			return m, tea.Quit
		}
		return m, nil
	case statusMsg:
		m.status = string(v)
		m.logs = append(m.logs, string(v))
		if len(m.logs) > 8 {
			m.logs = m.logs[len(m.logs)-8:]
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(v)
		return m, cmd
	case tea.KeyPressMsg:
		key := v.String()
		if m.editingHost {
			switch key {
			case "ctrl+c":
				m.editingHost = false
				if m.fontPreviewed {
					m.font = m.initialFont
					m.operation = "exit"
					return m, restoreFontPreview(m.initialFont)
				}
				return m, tea.Quit
			case "enter", "esc":
				m.editingHost = false
			case "backspace", "delete":
				if len(m.host) > 0 {
					m.host = m.host[:len(m.host)-1]
				}
			default:
				for _, r := range v.Text {
					if validHostRune(r) && len(m.host) < 32 {
						m.host += string(r)
					}
				}
			}
			return m, nil
		}
		if key == "ctrl+c" {
			if m.editingHost {
				m.editingHost = false
			}
			if m.fontPreviewed {
				m.font = m.initialFont
				m.operation = "exit"
				return m, restoreFontPreview(m.initialFont)
			}
			return m, tea.Quit
		}
		if key == "q" {
			if m.fontPreviewed {
				m.font = m.initialFont
				m.operation = "exit"
				return m, restoreFontPreview(m.initialFont)
			}
			return m, tea.Quit
		}
		if key == "esc" {
			switch m.page {
			case welcome:
				return m, tea.Quit
			case componentsPage:
				m.page = welcome
			case customizePage:
				m.page = finishedPage
				if m.fontPreviewed {
					m.font = m.initialFont
					m.operation = "cancel-preview"
					return m, previewFont(m.initialFont)
				}
			case restoreListPage:
				m.page = customizePage
			case restoreConfirmPage:
				m.page = restoreListPage
			case installingPage:
				if m.err == nil {
					return m, nil
				}
				if m.operation == "setup" {
					m.page = componentsPage
				} else {
					m.page = customizePage
				}
				m.err = nil
				m.status = ""
			default:
				return m, tea.Quit
			}
			return m, nil
		}
		switch m.page {
		case welcome:
			if key == "enter" {
				m.page = componentsPage
			}
		case componentsPage:
			switch key {
			case "up", "k":
				m.cursor = (m.cursor + len(m.components) - 1) % len(m.components)
			case "down", "j":
				m.cursor = (m.cursor + 1) % len(m.components)
			case "enter":
				m.page = installingPage
				m.operation = "setup"
				m.status = ""
				return m, install(m)
			}
		case customizePage:
			switch key {
			case "r":
				m.backups, _ = listBackups(os.Getenv("HOME"))
				m.backupCursor = 0
				m.page = restoreListPage
			case "left", "h":
				if m.cursor == 0 {
					m.theme = (m.theme + len(themes) - 1) % len(themes)
				} else if m.cursor == 1 {
					m.font = (m.font + len(fontNames) - 1) % len(fontNames)
					m.fontPreviewed = true
					return m, previewFont(m.font)
				} else if m.promptField == 0 {
					m.prompt = (m.prompt + len(promptThemes) - 1) % len(promptThemes)
				}
			case "right", "l":
				if m.cursor == 0 {
					m.theme = (m.theme + 1) % len(themes)
				} else if m.cursor == 1 {
					m.font = (m.font + 1) % len(fontNames)
					m.fontPreviewed = true
					return m, previewFont(m.font)
				} else if m.promptField == 0 {
					m.prompt = (m.prompt + 1) % len(promptThemes)
				}
			case "tab":
				m.cursor = (m.cursor + 1) % 3
			case "shift+tab":
				m.cursor = (m.cursor + 2) % 3
			case "up", "k", "down", "j":
				if m.cursor == 2 {
					m.promptField = 1 - m.promptField
				}
			case "e":
				if m.cursor == 2 && m.promptField == 1 {
					m.editingHost = true
				}
			case "enter", "a":
				m.page = installingPage
				m.operation = "apply"
				return m, applyOnly(m)
			}
		case reviewPage:
			if key == "backspace" {
				m.page = customizePage
			}
			if key == "enter" {
				m.page = installingPage
				m.operation = "setup"
				if m.customizeOnly {
					return m, applyOnly(m)
				}
				return m, install(m)
			}
		case installingPage:
			if m.err != nil {
				if key == "esc" {
					m.err = nil
					m.status = ""
					if m.operation == "setup" {
						m.page = componentsPage
					} else {
						m.page = customizePage
					}
					return m, nil
				}
				if key != "enter" {
					break
				}
				if m.operation == "restore" {
					m.page = restoreConfirmPage
				} else if m.operation == "setup" {
					m.page = componentsPage
				} else {
					m.page = customizePage
				}
				m.err = nil
				m.status = ""
			}
		case restoreListPage:
			switch key {
			case "esc":
				m.page = customizePage
			case "up", "k":
				if len(m.backups) > 0 {
					m.backupCursor = (m.backupCursor + len(m.backups) - 1) % len(m.backups)
				}
			case "down", "j":
				if len(m.backups) > 0 {
					m.backupCursor = (m.backupCursor + 1) % len(m.backups)
				}
			case "enter":
				if len(m.backups) > 0 {
					m.page = restoreConfirmPage
				}
			}
		case restoreConfirmPage:
			switch key {
			case "esc", "backspace":
				m.page = restoreListPage
			case "enter":
				m.page, m.operation, m.err, m.status, m.logs = installingPage, "restore", nil, "", nil
				return m, restoreBackup(os.Getenv("HOME"), os.Getenv("PREFIX"), m.backups[m.backupCursor])
			}
		case finishedPage:
			if key == "enter" || key == "q" {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}
func (m model) View() tea.View {
	accent := lipgloss.Color(themeAccent(m.theme))
	title := lipgloss.NewStyle().Bold(true).Foreground(accent).Render("HXTERMUX  /  TERMUX SETUP")
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#8792a8"))
	keyHint := func(key, action string) string {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#0b0d12")).Background(accent).Padding(0, 1).Render(key) + " " + muted.Render(action)
	}
	card := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#39445a")).Padding(1, 2)
	var body string
	switch m.page {
	case welcome:
		body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f0f3fa")).Render("A considered Termux setup.") + "\nInstall the complete HxTermux environment first. Personalize now or return to it later.\n\n" + keyHint("ENTER", "continue") + "   " + keyHint("Q / CTRL+C", "exit")
	case componentsPage:
		body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Render("01  FULL SETUP") + "\nAll components are required for the first setup. Personalization is available after installation or later with `hx`.\n\n"
		for i, c := range m.components {
			s := lipgloss.NewStyle().Foreground(lipgloss.Color("#dbe3f3"))
			if i == m.cursor {
				s = s.Foreground(accent).Bold(true)
			}
			body += s.Render("✓  "+c.name) + "\n    " + muted.Render(c.desc) + "\n"
		}
		body += "\n" + keyHint("ENTER", "install all") + "   " + keyHint("ESC", "back") + "   " + keyHint("Q / CTRL+C", "exit")
	case customizePage:
		headline := "Preview palette, font, and prompt, then confirm to apply."
		if m.customizeOnly {
			headline = "Preview palette, font, and prompt, then apply your changes."
		}
		body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Render("02  PERSONALIZE") + "\n" + headline + "\n\n"
		tabs := []string{"PALETTE", "FONT", "PROMPT"}
		for i, tab := range tabs {
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("#8792a8")).Padding(0, 1)
			if m.cursor == i {
				style = style.Foreground(lipgloss.Color("#0b0d12")).Background(accent).Bold(true)
			}
			body += style.Render(tab)
		}
		value := themes[m.theme]
		if m.cursor == 1 {
			value = fontNames[m.font]
		} else if m.cursor == 2 {
			if m.promptField == 0 {
				value = promptThemes[m.prompt]
			} else {
				value = "PROMPT HOST"
			}
		}
		body += "\n\n" + lipgloss.NewStyle().Foreground(accent).Bold(true).Render("‹  "+value+"  ›") + "\n"
		if m.cursor == 1 {
			body += lipgloss.NewStyle().Foreground(accent).Bold(true).Render("HxTermux  AaBb  012345") + "\n" + muted.Render("Font is reloaded in Termux as you browse.") + "\n"
		}
		if m.cursor == 2 {
			host := m.host
			if host == "" {
				host = "system hostname"
			}
			label := lipgloss.NewStyle().Foreground(lipgloss.Color("#dbe3f3"))
			if m.promptField == 1 {
				label = label.Foreground(accent).Bold(true)
			}
			body += "\n" + label.Render("Enter user / prompt host") + "  " + lipgloss.NewStyle().Foreground(accent).Render(host)
			if m.editingHost {
				body += lipgloss.NewStyle().Foreground(accent).Render("▏")
			}
			body += "\n"
		}
		if m.cursor == 0 {
			body += "\n"
			for i := 0; i < 16; i++ {
				c := paletteColor(m.theme, fmt.Sprintf("color%d", i))
				body += lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render("██")
			}
			body += "\n"
		}
		body += "\n" + keyHint("TAB", "switch tab") + "   " + keyHint("←/→", "change")
		if m.cursor == 2 && m.promptField == 1 {
			if m.editingHost {
				body += "   " + keyHint("TYPE", "host name") + "   " + keyHint("ENTER", "finish entry")
			} else {
				body += "   " + keyHint("E", "enter host")
			}
		}
		body += "\n" + keyHint("ENTER / A", "apply") + "   " + keyHint("ESC", "back / skip") + "   " + keyHint("Q / CTRL+C", "exit")
		if m.status != "" {
			body += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#ff7b72")).Render(m.status)
		}
	case reviewPage:
		reviewTitle, reviewPrompt := "03  READY TO INSTALL", "Enter install   Backspace edit"
		if m.customizeOnly {
			reviewTitle, reviewPrompt = "APPLY APPEARANCE", "Enter apply   Backspace edit"
		}
		body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Render(reviewTitle) + "\n\n"
		for _, c := range m.components {
			if c.selected {
				body += lipgloss.NewStyle().Foreground(accent).Render("✓  ") + c.name + "\n"
			}
		}
		body += "\nPalette  " + lipgloss.NewStyle().Foreground(accent).Render(themes[m.theme]) + "\nFont     " + muted.Render(fontNames[m.font]) + "\nPrompt   " + muted.Render(promptThemes[m.prompt]) + "\n\n" + muted.Render(reviewPrompt)
	case installingPage:
		heading, activity := "SETTING UP YOUR TERMUX", "Working through the selected components. This can take a few minutes."
		if m.operation == "restore" {
			heading, activity = "RESTORING CONFIGURATION", "Saving the current setup, then restoring the selected snapshot."
		}
		body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Render(heading) + "\n\n" + lipgloss.NewStyle().Foreground(accent).Render(m.spinner.View()) + "  " + muted.Render(activity) + "\n"
		for _, l := range m.logs[max(0, len(m.logs)-5):] {
			body += "\n  " + lipgloss.NewStyle().Foreground(accent).Render("› ") + l
		}
		if m.status != "" {
			body += "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#ff7b72")).Render(m.status) + "\n" + keyHint("ENTER", "retry / continue") + "   " + keyHint("ESC", "back") + "   " + keyHint("Q / CTRL+C", "exit")
		}
	case finishedPage:
		if m.operation == "restore" {
			body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Foreground(accent).Render("Configuration restored.") + "\nThe current setup was saved as a safety snapshot before restoring.\n\n" + strings.Join(m.logs, "\n") + "\n\n" + muted.Render("Termux packages remain installed; configuration and HxTermux-managed files were restored.") + "\n\n" + keyHint("ENTER", "close") + "   " + keyHint("Q / CTRL+C", "exit")
		} else if m.operation == "setup" {
			body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Foreground(accent).Render("Setup complete.") + "\nHxTermux is ready. Run `exec zsh` to start Zsh now, `hx` to customize later, or `hxf` to see HypexFetch.\n\n" + muted.Render("Your selected terminal palette and font are now active.") + "\n\n" + keyHint("ENTER", "close") + "   " + keyHint("Q / CTRL+C", "exit")
		} else if m.customizeOnly {
			body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Foreground(accent).Render("Appearance saved.") + "\nPalette and font are active now. Prompt changes load when Zsh starts.\n\n" + muted.Render("Run `exec zsh` to reload the prompt, or open a new Termux session.") + "\n\n" + keyHint("ENTER", "close") + "   " + keyHint("Q / CTRL+C", "exit")
		} else {
			body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Foreground(accent).Render("Setup complete.") + "\nHxTermux is ready. Run `exec zsh` to start Zsh now, `hx` to customize later, or `hxf` to see HypexFetch.\n\n" + muted.Render("Your selected terminal palette and font are now active.") + "\n\n" + keyHint("ENTER", "close") + "   " + keyHint("Q / CTRL+C", "exit")
		}
	case restoreListPage:
		body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Render("RESTORE A CONFIGURATION") + "\nChoose a saved snapshot. Restoring also creates a snapshot of the current setup.\n\n"
		if len(m.backups) == 0 {
			body += muted.Render("No HxTermux snapshots found. Install HxTermux once to create a restore point.")
		} else {
			for i, b := range m.backups {
				marker, style := "  ", lipgloss.NewStyle().Foreground(lipgloss.Color("#dbe3f3"))
				if i == m.backupCursor {
					marker, style = "› ", lipgloss.NewStyle().Foreground(accent).Bold(true)
				}
				detail := fmt.Sprintf("%s · %d saved paths", b.Kind, b.Entries)
				if !b.Complete {
					detail += " · partial legacy backup"
				}
				body += style.Render(marker+b.ID) + "\n    " + muted.Render(detail) + "\n"
			}
		}
		body += "\n" + keyHint("↑/↓", "select") + "   " + keyHint("ENTER", "continue") + "   " + keyHint("ESC", "back") + "   " + keyHint("Q / CTRL+C", "exit")
	case restoreConfirmPage:
		b := m.backups[m.backupCursor]
		body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Render("CONFIRM RESTORE") + "\n\nRestore snapshot " + lipgloss.NewStyle().Foreground(accent).Bold(true).Render(b.ID) + "?\n\nThe current managed files will first be copied to a new safety snapshot.\nPackages will remain installed.\n\n" + keyHint("ENTER", "restore") + "   " + keyHint("ESC", "back") + "   " + keyHint("Q / CTRL+C", "exit")
	}
	width := max(24, min(m.width-2, 86))
	content := card.Width(max(18, width-8)).Render(body)
	view := tea.NewView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content))
	view.AltScreen = true
	view.BackgroundColor = lipgloss.Color("#0b0d12")
	return view
}

func install(m model) tea.Cmd {
	return func() tea.Msg {
		lines := []string{}
		log := func(s string) {
			lines = append(lines, s)
			if activeProgram != nil {
				activeProgram.Send(statusMsg(s))
			}
		}
		fail := func(e error) tea.Msg { return installMsg{lines, e} }
		home := os.Getenv("HOME")
		root, err := findSourceRoot(home)
		if err != nil {
			return fail(err)
		}
		prefix := os.Getenv("PREFIX")
		if prefix == "" {
			prefix = "/data/data/com.termux/files/usr"
		}
		log("Creating pre-install restore point")
		activeSnapshot = ""
		activeSnapshot, err = createSnapshot(home, prefix, "setup")
		if err != nil {
			return fail(err)
		}
		log("Restore point saved: " + filepath.Base(activeSnapshot))
		selected := map[string]bool{}
		for _, c := range m.components {
			selected[c.name] = c.selected
		}
		if selected["Core shell tools"] {
			log("Installing shell tools")
			if err = run("pkg", "install", "-y", "git", "curl", "eza", "fzf", "lf", "tmux", "zsh", "bat", "clang", "termux-api"); err != nil {
				return fail(err)
			}
		}
		if selected["HxTermux dotfiles"] {
			log("Backing up and installing dotfiles")
			if err = copyDotfiles(root, home); err != nil {
				return fail(err)
			}
		}
		if selected["Zsh experience"] {
			if err = installZshConfig(root, home); err != nil {
				return fail(err)
			}
		}
		data := filepath.Join(home, ".local/share/hxtermux")
		if err = os.MkdirAll(filepath.Join(data, "themes"), 0755); err != nil {
			return fail(err)
		}
		if err = os.MkdirAll(filepath.Join(data, "fonts"), 0755); err != nil {
			return fail(err)
		}
		if err = copyDir(filepath.Join(root, ".colorscheme"), filepath.Join(data, "themes")); err != nil {
			return fail(err)
		}
		if err = copyDir(filepath.Join(root, ".fonts"), filepath.Join(data, "fonts")); err != nil {
			return fail(err)
		}
		if selected["Zsh experience"] {
			log("Installing Zsh and plugins")
			if err = run("pkg", "install", "-y", "git", "zsh", "termux-tools"); err != nil {
				return fail(err)
			}
			if err = ensureRepoEntry("https://github.com/ohmyzsh/ohmyzsh.git", filepath.Join(home, ".oh-my-zsh"), "oh-my-zsh.sh"); err != nil {
				return fail(err)
			}
			if err = copyDir(filepath.Join(root, ".oh-my-zsh/custom/themes"), filepath.Join(home, ".oh-my-zsh/custom/themes")); err != nil {
				return fail(err)
			}
			plugins := filepath.Join(home, ".oh-my-zsh/custom/plugins")
			for _, p := range []struct{ u, n, entry string }{
				{"https://github.com/zsh-users/zsh-autosuggestions.git", "zsh-autosuggestions", "zsh-autosuggestions.plugin.zsh"},
				{"https://github.com/zsh-users/zsh-syntax-highlighting.git", "zsh-syntax-highlighting", "zsh-syntax-highlighting.plugin.zsh"},
				{"https://github.com/joshskidmore/zsh-fzf-history-search.git", "zsh-fzf-history-search", "zsh-fzf-history-search.plugin.zsh"},
				{"https://github.com/marlonrichert/zsh-autocomplete.git", "zsh-autocomplete", "zsh-autocomplete.plugin.zsh"},
			} {
				log("Initializing Zsh plugin repository: " + p.n)
				if err = ensureRepoEntry(p.u, filepath.Join(plugins, p.n), p.entry); err != nil {
					return fail(err)
				}
			}
			log("Setting Zsh as the Termux login shell")
			if err = run("chsh", "-s", "zsh"); err != nil {
				return fail(err)
			}
		}
		if selected["HypexFetch"] {
			log("Building HypexFetch")
			dest := filepath.Join(prefix, "bin/hypexfetch")
			prebuilt := filepath.Join(root, "hypexfetch")
			if _, statErr := os.Stat(prebuilt); statErr == nil {
				err = copyFile(prebuilt, dest, 0755)
			} else {
				fetch := filepath.Join(root, "fetcher")
				err = runIn(fetch, "go", "build", "-o", dest, ".")
			}
			if err != nil {
				return fail(err)
			}
		}
		if selected["Awesomeshot"] {
			log("Installing Awesomeshot from its Termux branch")
			if err = run("pkg", "install", "-y", "termux-api", "imagemagick", "inotify-tools", "bc", "make", "ncurses-utils", "git"); err != nil {
				return fail(err)
			}
			dest := filepath.Join(data, "awesomeshot")
			if err = clone("https://github.com/Awesomesh0t/awesomeshot.git", dest, "--branch", "termux", "--depth", "1"); err != nil {
				return fail(err)
			}
			if err = runIn(dest, "make", "install"); err != nil {
				return fail(err)
			}
		}
		if selected["Neovim starter"] {
			log("Installing Neovim and its plugin build tools")
			if err = run("pkg", "install", "-y", "git", "neovim", "nodejs", "ripgrep", "unzip", "make", "clang"); err != nil {
				return fail(err)
			}
			if _, e := os.Stat(filepath.Join(home, ".config/nvim")); e == nil {
				if e = backup(filepath.Join(home, ".config/nvim")); e != nil {
					return fail(e)
				}
			}
			nvimDir := filepath.Join(home, ".config/nvim")
			if err = clone("https://github.com/NvChad/starter.git", nvimDir, "--depth=1"); err != nil {
				return fail(err)
			}
			customConfig := filepath.Join(root, "optional/neovim-settings/xshin.lua")
			if err = copyFile(customConfig, filepath.Join(nvimDir, "lua/configs/hxtermux.lua"), 0644); err != nil {
				return fail(err)
			}
			initFile := filepath.Join(nvimDir, "init.lua")
			initData, readErr := os.ReadFile(initFile)
			if readErr != nil {
				return fail(readErr)
			}
			if !strings.Contains(string(initData), `require "configs.hxtermux"`) {
				initData = append(initData, []byte("\nrequire \"configs.hxtermux\"\n")...)
				if err = os.WriteFile(initFile, initData, 0644); err != nil {
					return fail(err)
				}
			}
			log("Running the first NvChad plugin sync")
			if err = run("nvim", "--headless", "+Lazy! sync", "+qa"); err != nil {
				return fail(fmt.Errorf("Neovim first-run plugin sync failed: %w", err))
			}
		}
		if err = applyAppearance(root, home, m.theme, m.font, m.prompt, m.host); err != nil {
			return fail(err)
		}
		if selected["Zsh experience"] {
			if err = applyPromptTheme(home, promptThemes[m.prompt], m.host); err != nil {
				return fail(err)
			}
		}
		log("Installing HxTermux command")
		dest := filepath.Join(prefix, "bin/hxtermux")
		prebuilt := filepath.Join(root, "hxtermux")
		if _, statErr := os.Stat(prebuilt); statErr == nil {
			err = copyFile(prebuilt, dest, 0755)
		} else {
			err = runIn(root, "go", "build", "-o", dest, ".")
		}
		if err != nil {
			return fail(err)
		}
		marker := filepath.Join(home, ".config/hxtermux/setup-complete")
		if err = os.WriteFile(marker, []byte(time.Now().Format(time.RFC3339)+"\n"), 0644); err != nil {
			return fail(err)
		}
		return installMsg{lines: append(lines, "All selected components installed"), err: nil}
	}
}

func findSourceRoot(home string) (string, error) {
	var candidates []string
	if root := os.Getenv("HXTERMUX_SOURCE"); root != "" {
		candidates = append(candidates, root)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	candidates = append(candidates,
		filepath.Join(home, ".cache/hxtermux/source"),
		filepath.Join(home, ".local/share/hxtermux/source"),
	)
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(executable))
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, ".colorscheme")); err == nil {
			if _, err = os.Stat(filepath.Join(candidate, ".fonts")); err == nil {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("HxTermux setup assets were not found; run install.sh from the HxTermux source folder or reinstall from the one-line installer")
}

func run(name string, args ...string) error {
	c := exec.Command(name, args...)
	return runCommand(c)
}
func runIn(dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	return runCommand(c)
}
func runCommand(c *exec.Cmd) error {
	out, err := c.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("%s: %w: %s", filepath.Base(c.Path), err, detail)
		}
		return fmt.Errorf("%s: %w", filepath.Base(c.Path), err)
	}
	return nil
}
func clone(url, dest string, args ...string) error {
	if _, e := os.Stat(dest); e == nil {
		if exec.Command("git", "-C", dest, "rev-parse", "--is-inside-work-tree").Run() == nil {
			return nil
		}
		if e = backup(dest); e != nil {
			return e
		}
	}
	if e := os.MkdirAll(filepath.Dir(dest), 0755); e != nil {
		return e
	}
	a := append([]string{"clone"}, args...)
	a = append(a, url, dest)
	return run("git", a...)
}

// ensureRepoEntry initializes a repository when it is absent and checks that
// an existing checkout contains the entrypoint expected by the shell setup.
func ensureRepoEntry(url, dest, entry string) error {
	entryPath := filepath.Join(dest, entry)
	if info, err := os.Stat(entryPath); err == nil && !info.IsDir() {
		return nil
	}
	if _, err := os.Stat(dest); err == nil {
		if err = backup(dest); err != nil {
			return err
		}
	}
	if err := clone(url, dest, "--depth=1"); err != nil {
		return err
	}
	if info, err := os.Stat(entryPath); err != nil || info.IsDir() {
		return fmt.Errorf("plugin %s initialized without expected entrypoint %s", filepath.Base(dest), entry)
	}
	return nil
}

type snapshotEntry struct {
	Scope  string `json:"scope"`
	Path   string `json:"path"`
	Stored string `json:"stored,omitempty"`
}
type snapshotManifest struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Created  time.Time       `json:"created"`
	Complete bool            `json:"complete"`
	Entries  []snapshotEntry `json:"entries"`
}
type managedTarget struct{ Scope, Path, Absolute string }

func backup(path string) error {
	// The complete pre-install snapshot is already safely copied before any
	// HxTermux files are replaced. Remove old destinations so copies are clean.
	if activeSnapshot != "" {
		return os.RemoveAll(path)
	}
	return fmt.Errorf("refusing to replace %s without an active HxTermux snapshot", path)
}

func managedTargets(home, prefix string) []managedTarget {
	targets := []managedTarget{}
	for _, rel := range []string{".aliases", ".autostart", ".config", ".colorscheme", ".fonts", ".local", ".oh-my-zsh", ".scripts", ".termux", ".tmux.conf", ".zshrc"} {
		targets = append(targets, managedTarget{"home", rel, filepath.Join(home, rel)})
	}
	if prefix != "" {
		// Keep hxtermux itself available so the user can restore a safety
		// snapshot even after returning to a pre-install state.
		for _, rel := range []string{"bin/hypexfetch"} {
			targets = append(targets, managedTarget{"prefix", rel, filepath.Join(prefix, rel)})
		}
	}
	return targets
}

func backupRoot(home string) string { return filepath.Join(home, ".hxtermux-backups") }

func createSnapshot(home, prefix, kind string) (string, error) {
	root := backupRoot(home)
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	now := time.Now().UTC()
	id := now.Format("20060102-150405.000000000") + "-" + kind
	dir := filepath.Join(root, id)
	if err := os.Mkdir(dir, 0700); err != nil {
		return "", err
	}
	manifest := snapshotManifest{ID: id, Kind: kind, Created: now, Complete: true, Entries: []snapshotEntry{}}
	for _, target := range managedTargets(home, prefix) {
		if _, err := os.Lstat(target.Absolute); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			_ = os.RemoveAll(dir)
			return "", err
		}
		stored := filepath.Join("files", target.Scope, filepath.FromSlash(target.Path))
		if err := copyPath(target.Absolute, filepath.Join(dir, stored)); err != nil {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("snapshot %s: %w", target.Absolute, err)
		}
		manifest.Entries = append(manifest.Entries, snapshotEntry{Scope: target.Scope, Path: filepath.ToSlash(target.Path)})
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	if err = os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0600); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

func listBackups(home string) ([]backupInfo, error) {
	entries, err := os.ReadDir(backupRoot(home))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	backups := []backupInfo{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(backupRoot(home), entry.Name())
		manifest, err := readSnapshot(dir)
		if err != nil {
			continue
		}
		backups = append(backups, backupInfo{ID: entry.Name(), Kind: manifest.Kind, Created: manifest.Created, Entries: len(manifest.Entries), Complete: manifest.Complete})
	}
	// Also surface backups from the earlier installer, which stored them beside
	// each dotfile as <name>.<timestamp>.backup.
	children, err := os.ReadDir(home)
	if err != nil {
		return nil, err
	}
	legacyRoots := []string{".aliases", ".autostart", ".config", ".colorscheme", ".fonts", ".local", ".oh-my-zsh", ".scripts", ".termux", ".tmux.conf", ".zshrc"}
	for _, child := range children {
		for _, base := range legacyRoots {
			prefix, suffix := base+".", ".backup"
			if strings.HasPrefix(child.Name(), prefix) && strings.HasSuffix(child.Name(), suffix) && len(child.Name()) > len(prefix)+len(suffix) {
				info, _ := child.Info()
				created := time.Time{}
				if info != nil {
					created = info.ModTime()
				}
				backups = append(backups, backupInfo{ID: child.Name(), Kind: "legacy backup", Created: created, Entries: 1, SourcePath: filepath.Join(home, child.Name()), LegacyScope: "home", LegacyRel: base})
				break
			}
		}
	}
	sort.Slice(backups, func(i, j int) bool { return backups[i].ID > backups[j].ID })
	return backups, nil
}

func readSnapshot(dir string) (snapshotManifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err == nil {
		var m snapshotManifest
		if err = json.Unmarshal(data, &m); err != nil {
			return snapshotManifest{}, err
		}
		return m, nil
	}
	if !os.IsNotExist(err) {
		return snapshotManifest{}, err
	}
	// Accept backups made by the earlier HxTermux build, which stored a
	// timestamp directory containing the replaced path by basename.
	children, err := os.ReadDir(dir)
	if err != nil {
		return snapshotManifest{}, err
	}
	m := snapshotManifest{ID: filepath.Base(dir), Kind: "legacy backup", Created: time.Now(), Entries: []snapshotEntry{}}
	for _, child := range children {
		if child.Name() == "manifest.json" {
			continue
		}
		scope, rel, ok := legacyBackupPath(child.Name())
		if !ok {
			continue
		}
		m.Entries = append(m.Entries, snapshotEntry{Scope: scope, Path: rel, Stored: child.Name()})
	}
	if len(m.Entries) == 0 {
		return snapshotManifest{}, fmt.Errorf("no recognized snapshot entries")
	}
	return m, nil
}

func legacyBackupPath(name string) (scope, rel string, ok bool) {
	switch name {
	case ".aliases", ".autostart", ".colorscheme", ".fonts", ".local", ".oh-my-zsh", ".scripts", ".termux", ".tmux.conf", ".zshrc":
		return "home", name, true
	case ".config":
		return "home", ".config", true
	case "lf":
		return "home", ".config/lf", true
	case "nvim":
		return "home", ".config/nvim", true
	case "hxtermux":
		return "home", ".config/hxtermux", true
	default:
		return "", "", false
	}
}

func restoreBackup(home, prefix string, selected backupInfo) tea.Cmd {
	return func() tea.Msg {
		log := func(s string) {
			if activeProgram != nil {
				activeProgram.Send(statusMsg(s))
			}
		}
		preRestore, err := createSnapshot(home, prefix, "pre-restore")
		if err != nil {
			return installMsg{err: fmt.Errorf("could not save the current setup: %w", err)}
		}
		log("Safety snapshot saved: " + filepath.Base(preRestore))
		dir := filepath.Join(backupRoot(home), selected.ID)
		var manifest snapshotManifest
		if selected.SourcePath != "" {
			dir = home
			manifest = snapshotManifest{ID: selected.ID, Kind: "legacy backup", Entries: []snapshotEntry{{Scope: selected.LegacyScope, Path: selected.LegacyRel, Stored: filepath.Base(selected.SourcePath)}}}
		} else {
			manifest, err = readSnapshot(dir)
			if err != nil {
				return installMsg{err: err}
			}
		}
		log("Restoring the saved stock configuration")
		if err = applySnapshot(home, prefix, dir, manifest); err != nil {
			rollbackManifest, readErr := readSnapshot(preRestore)
			if readErr == nil {
				if rollbackErr := applySnapshot(home, prefix, preRestore, rollbackManifest); rollbackErr != nil {
					return installMsg{err: fmt.Errorf("restore failed (%v); automatic rollback also failed (%v). Safety snapshot: %s", err, rollbackErr, filepath.Base(preRestore))}
				}
			}
			return installMsg{err: fmt.Errorf("restore failed and the current setup was recovered from its safety snapshot: %w", err)}
		}
		if command, lookupErr := exec.LookPath("termux-reload-settings"); lookupErr == nil {
			_ = exec.Command(command).Run()
		}
		return installMsg{lines: []string{"Restored: " + selected.ID, "Safety snapshot: " + filepath.Base(preRestore)}}
	}
}

func applySnapshot(home, prefix, dir string, manifest snapshotManifest) error {
	targets := managedTargets(home, prefix)
	type restoreItem struct{ target, source string }
	items := make([]restoreItem, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		target, err := resolveSnapshotEntry(home, prefix, entry, targets)
		if err != nil {
			return err
		}
		sourceRel := entry.Stored
		if sourceRel == "" {
			sourceRel = filepath.Join("files", entry.Scope, filepath.FromSlash(entry.Path))
		}
		cleanSource := filepath.Clean(sourceRel)
		if filepath.IsAbs(cleanSource) || cleanSource == ".." || strings.HasPrefix(cleanSource, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid snapshot file path")
		}
		source := filepath.Join(dir, cleanSource)
		if _, err = os.Lstat(source); err != nil {
			return fmt.Errorf("snapshot is incomplete at %s: %w", source, err)
		}
		items = append(items, restoreItem{target, source})
	}
	if manifest.Complete {
		for _, target := range targets {
			if err := os.RemoveAll(target.Absolute); err != nil {
				return err
			}
		}
	} else {
		// Older backups represented one replaced path, not a full restore point.
		// Restore only those saved paths so other current configs remain intact.
		for _, item := range items {
			if err := os.RemoveAll(item.target); err != nil {
				return err
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return strings.Count(items[i].target, string(filepath.Separator)) < strings.Count(items[j].target, string(filepath.Separator))
	})
	for _, item := range items {
		if err := copyPath(item.source, item.target); err != nil {
			return fmt.Errorf("restore %s: %w", item.target, err)
		}
	}
	return nil
}

func resolveSnapshotEntry(home, prefix string, entry snapshotEntry, targets []managedTarget) (string, error) {
	rel := filepath.Clean(filepath.FromSlash(entry.Path))
	if rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path in snapshot")
	}
	base := home
	if entry.Scope == "prefix" {
		base = prefix
	} else if entry.Scope != "home" {
		return "", fmt.Errorf("invalid snapshot scope %q", entry.Scope)
	}
	if base == "" {
		return "", fmt.Errorf("snapshot requires a Termux PREFIX")
	}
	target := filepath.Join(base, rel)
	for _, managed := range targets {
		if managed.Absolute == target || strings.HasPrefix(target, managed.Absolute+string(filepath.Separator)) {
			return target, nil
		}
	}
	return "", fmt.Errorf("snapshot path is outside HxTermux-managed configuration: %s", rel)
}
func copyDotfiles(root, home string) error {
	for _, n := range []string{".aliases", ".autostart", ".colorscheme", ".config/lf", ".fonts", ".local", ".scripts", ".termux", ".tmux.conf"} {
		src := filepath.Join(root, n)
		if _, e := os.Stat(src); e != nil {
			continue
		}
		dst := filepath.Join(home, n)
		if n != ".local" {
			if _, e := os.Stat(dst); e == nil {
				if e = backup(dst); e != nil {
					return e
				}
			}
		}
		if e := copyPath(src, dst); e != nil {
			return e
		}
	}
	return nil
}
func installZshConfig(root, home string) error {
	src, dst := filepath.Join(root, ".zshrc"), filepath.Join(home, ".zshrc")
	if _, err := os.Stat(dst); err == nil {
		if err = backup(dst); err != nil {
			return err
		}
	}
	return copyPath(src, dst)
}
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.Mode()&os.ModeSymlink != 0 {
			return copyPath(p, target)
		}
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(p, target, info.Mode())
	})
}
func copyPath(src, dst string) error {
	s, e := os.Lstat(src)
	if e != nil {
		return e
	}
	if s.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		if err = os.RemoveAll(dst); err != nil {
			return err
		}
		return os.Symlink(link, dst)
	}
	if s.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst, s.Mode())
}
func copyFile(src, dst string, mode os.FileMode) error {
	if e := os.MkdirAll(filepath.Dir(dst), 0755); e != nil {
		return e
	}
	a, e := os.ReadFile(src)
	if e != nil {
		return e
	}
	return os.WriteFile(dst, a, mode.Perm())
}
func applyAppearance(root, home string, theme, font, prompt int, host string) error {
	t := themes[theme%len(themes)]
	fontName := fontNames[font%len(fontNames)]
	assetRoot := root
	if root == "" {
		assetRoot = filepath.Join(home, ".local/share/hxtermux")
	}
	themeDir, fontDir := filepath.Join(assetRoot, ".colorscheme"), filepath.Join(assetRoot, ".fonts")
	if root == "" {
		themeDir, fontDir = filepath.Join(assetRoot, "themes"), filepath.Join(assetRoot, "fonts")
	}
	dir := filepath.Join(home, ".termux")
	if e := os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	if e := copyFile(filepath.Join(themeDir, t+".colors"), filepath.Join(dir, "colors.properties"), 0644); e != nil {
		return e
	}
	if e := copyFile(filepath.Join(fontDir, fontName), filepath.Join(dir, "font.ttf"), 0644); e != nil {
		return e
	}
	prefs := fmt.Sprintf("theme=%s\nfont=%s\nprompt=%s\nhost=%s\n", t, fontName, promptThemes[prompt%len(promptThemes)], host)
	if e := os.MkdirAll(filepath.Join(home, ".config/hxtermux"), 0755); e != nil {
		return e
	}
	if e := os.WriteFile(filepath.Join(home, ".config/hxtermux/preferences"), []byte(prefs), 0644); e != nil {
		return e
	}
	if exe, e := exec.LookPath("termux-reload-settings"); e == nil {
		return exec.Command(exe).Run()
	}
	return nil
}

func applyPromptTheme(home, prompt, host string) error {
	valid := false
	for _, candidate := range promptThemes {
		if candidate == prompt {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("unknown prompt theme %q", prompt)
	}
	for _, r := range host {
		if !validHostRune(r) {
			return fmt.Errorf("host name may only contain letters, numbers, dots, dashes, and underscores")
		}
	}
	path := filepath.Join(home, ".zshrc")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot set the prompt theme before installing the Zsh experience: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	found := false
	hostFound := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ZSH_THEME=") {
			if prompt == "none" {
				lines[i] = `ZSH_THEME=""`
			} else {
				lines[i] = fmt.Sprintf("ZSH_THEME=%q", prompt)
			}
			found = true
		}
		if strings.HasPrefix(trimmed, "export HXTERMUX_HOST=") {
			lines[i] = "export HXTERMUX_HOST=" + shellQuote(host)
			hostFound = true
		}
	}
	if !found {
		return fmt.Errorf("could not find ZSH_THEME in %s", path)
	}
	if !hostFound {
		lines = append([]string{"export HXTERMUX_HOST=" + shellQuote(host)}, lines...)
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), info.Mode().Perm())
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func paletteColor(theme int, key string) string {
	fallback := colors[theme%len(colors)]
	name := themes[theme%len(themes)] + ".colors"
	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".local/share/hxtermux/themes", name))
	if err != nil {
		data, err = paletteFiles.ReadFile(filepath.Join(".colorscheme", name))
	}
	if err != nil {
		return fallback
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		sep := strings.IndexAny(line, "=:")
		if sep < 0 || strings.TrimSpace(line[:sep]) != key {
			continue
		}
		value := strings.TrimSpace(line[sep+1:])
		if len(value) == 7 && value[0] == '#' {
			return value
		}
	}
	return fallback
}

func themeAccent(theme int) string { return paletteColor(theme, "color4") }
