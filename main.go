package main

import (
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
	page                               page
	width, height, cursor, theme, font int
	components                         []component
	status                             string
	logs                               []string
	err                                error
	started                            bool
	customizeOnly                      bool
	spinner                            spinner.Model
	backups                            []backupInfo
	backupCursor                       int
	operation                          string
}
type installMsg struct {
	lines []string
	err   error
}
type statusMsg string
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
	theme, font := savedAppearance()
	m := model{page: startPage, theme: theme, font: font, customizeOnly: !*setup, spinner: spinner.New(spinner.WithSpinner(spinner.Dot)), components: []component{
		{"Core shell tools", "git · curl · eza · fzf · lf · tmux · zsh · Termux:API", true},
		{"HxTermux dotfiles", "Shell, terminal settings, aliases and helper scripts", true},
		{"Zsh experience", "Oh My Zsh with autosuggestions and syntax highlighting", true},
		{"HypexFetch", "Your animated Bubble Tea system fetch", true},
		{"Awesomeshot", "Screenshot utility from its official Termux source branch", false},
		{"Neovim starter", "NvChad starter configuration", false},
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

func savedAppearance() (int, int) {
	themeIndex, fontIndex := 1, 0
	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config/hxtermux/preferences"))
	if err != nil {
		return themeIndex, fontIndex
	}
	var themeName, fontName string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "theme=") {
			themeName = strings.TrimPrefix(line, "theme=")
		}
		if strings.HasPrefix(line, "font=") {
			fontName = strings.TrimPrefix(line, "font=")
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
	return themeIndex, fontIndex
}

var fontNames = []string{"Fira Code Bold Nerd Font.ttf", "Fira Code Medium Nerd Font Complete Mono.ttf", "JetBrains Mono Bold Nerd Font Complete.ttf", "JetBrains Mono Medium Nerd Font Complete.ttf", "MesloLGS NF Bold Italic.ttf", "MesloLGS NF Bold.ttf", "MesloLGS NF Italic.ttf", "MesloLGS NF Regular.ttf"}

func applyOnly(m model) tea.Cmd {
	return func() tea.Msg {
		err := applyAppearance("", os.Getenv("HOME"), m.theme, m.font)
		if err != nil {
			return installMsg{err: err}
		}
		return installMsg{lines: []string{"Appearance saved"}}
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
			m.page = finishedPage
		} else {
			m.status = v.err.Error()
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
		if key == "ctrl+c" {
			return m, tea.Quit
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
			case " ":
				m.components[m.cursor].selected = !m.components[m.cursor].selected
			case "enter":
				m.page = customizePage
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
				} else {
					m.font = (m.font + len(fontNames) - 1) % len(fontNames)
				}
			case "right", "l":
				if m.cursor == 0 {
					m.theme = (m.theme + 1) % len(themes)
				} else {
					m.font = (m.font + 1) % len(fontNames)
				}
			case "up", "k", "down", "j":
				m.cursor = (m.cursor + 1) % 2
			case "enter":
				m.page = reviewPage
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
			if m.err != nil && key == "enter" {
				if m.operation == "restore" {
					m.page = restoreConfirmPage
				} else {
					m.page = reviewPage
				}
				m.err = nil
				m.status = ""
			}
		case restoreListPage:
			switch key {
			case "esc", "q":
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
	accent := lipgloss.Color(colors[m.theme%len(colors)])
	title := lipgloss.NewStyle().Bold(true).Foreground(accent).Render("HXTERMUX  /  TERMUX SETUP")
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#8792a8"))
	card := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#39445a")).Padding(1, 2)
	var body string
	switch m.page {
	case welcome:
		body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f0f3fa")).Render("A considered Termux setup.") + "\nChoose your shell tools, shape the terminal, and install your HypexFetch build in one guided flow.\n\n" + muted.Render("Responsive Bubble Tea interface · review every choice before installation") + "\n\n" + lipgloss.NewStyle().Foreground(accent).Render("Enter  Begin setup")
	case componentsPage:
		body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Render("01  COMPONENTS") + "\nPick what belongs in your environment. Space toggles a component.\n\n"
		for i, c := range m.components {
			marker := "○"
			if c.selected {
				marker = "●"
			}
			s := lipgloss.NewStyle().Foreground(lipgloss.Color("#dbe3f3"))
			if i == m.cursor {
				s = s.Foreground(accent).Bold(true)
			}
			body += s.Render(marker+"  "+c.name) + "\n    " + muted.Render(c.desc) + "\n"
		}
		body += "\n" + muted.Render("↑/↓ move   Space select   Enter customize")
	case customizePage:
		headline := "Theme and font are applied after you confirm the installation."
		if m.customizeOnly {
			headline = "Preview a palette and font, then apply your changes."
		}
		body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Render("02  PERSONALIZE") + "\n" + headline + "\n\n"
		for i, label := range []string{"Terminal palette", "Terminal font"} {
			s := lipgloss.NewStyle().Foreground(lipgloss.Color("#dbe3f3"))
			if m.cursor == i {
				s = s.Foreground(accent).Bold(true)
			}
			value := themes[m.theme]
			if i == 1 {
				value = fontNames[m.font]
			}
			body += s.Render("◆  "+label) + "\n    " + lipgloss.NewStyle().Foreground(accent).Render(value) + "\n"
		}
		body += "\n" + strings.Join([]string{lipgloss.NewStyle().Foreground(lipgloss.Color("#11131a")).Background(accent).Render("  "), lipgloss.NewStyle().Foreground(accent).Render("● ● ● ● ● ● ● ●")}, " ") + "\n\n" + muted.Render("↑/↓ choose field   ←/→ preview   Enter review   r restore")
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
		body += "\nPalette  " + lipgloss.NewStyle().Foreground(accent).Render(themes[m.theme]) + "\nFont     " + muted.Render(fontNames[m.font]) + "\n\n" + muted.Render(reviewPrompt)
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
			body += "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#ff7b72")).Render(m.status+"\nEnter to return to review")
		}
	case finishedPage:
		if m.operation == "restore" {
			body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Foreground(accent).Render("Configuration restored.") + "\nThe current setup was saved as a safety snapshot before restoring.\n\n" + strings.Join(m.logs, "\n") + "\n\n" + muted.Render("Termux packages remain installed; configuration and HxTermux-managed files were restored.") + "\n\nEnter to close"
		} else {
			body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Foreground(accent).Render("Setup complete.") + "\nHxTermux is ready. Run `hxtermux` to reopen this studio or `hypexfetch` to see your fetch.\n\n" + muted.Render("Your selected terminal palette and font are now active.") + "\n\nEnter to close"
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
		body += "\n" + muted.Render("↑/↓ select   Enter continue   Esc back")
	case restoreConfirmPage:
		b := m.backups[m.backupCursor]
		body = title + "\n\n" + lipgloss.NewStyle().Bold(true).Render("CONFIRM RESTORE") + "\n\nRestore snapshot " + lipgloss.NewStyle().Foreground(accent).Bold(true).Render(b.ID) + "?\n\nThe current managed files will first be copied to a new safety snapshot.\nPackages will remain installed.\n\n" + lipgloss.NewStyle().Foreground(accent).Render("Enter restore   Esc cancel")
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
		root, err := os.Getwd()
		if err != nil {
			return fail(err)
		}
		home := os.Getenv("HOME")
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
			if err = run("pkg", "install", "-y", "git", "zsh"); err != nil {
				return fail(err)
			}
			if err = clone("https://github.com/ohmyzsh/ohmyzsh.git", filepath.Join(home, ".oh-my-zsh")); err != nil {
				return fail(err)
			}
			if err = copyDir(filepath.Join(root, ".oh-my-zsh/custom/themes"), filepath.Join(home, ".oh-my-zsh/custom/themes")); err != nil {
				return fail(err)
			}
			plugins := filepath.Join(home, ".oh-my-zsh/custom/plugins")
			for _, p := range []struct{ u, n string }{{"https://github.com/zsh-users/zsh-autosuggestions.git", "zsh-autosuggestions"}, {"https://github.com/zsh-users/zsh-syntax-highlighting.git", "zsh-syntax-highlighting"}, {"https://github.com/joshskidmore/zsh-fzf-history-search.git", "zsh-fzf-history-search"}, {"https://github.com/marlonrichert/zsh-autocomplete.git", "zsh-autocomplete"}} {
				if err = clone(p.u, filepath.Join(plugins, p.n)); err != nil {
					return fail(err)
				}
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
			log("Installing Neovim and starter config")
			if err = run("pkg", "install", "-y", "git", "neovim"); err != nil {
				return fail(err)
			}
			if _, e := os.Stat(filepath.Join(home, ".config/nvim")); e == nil {
				if e = backup(filepath.Join(home, ".config/nvim")); e != nil {
					return fail(e)
				}
			}
			if err = clone("https://github.com/NvChad/starter.git", filepath.Join(home, ".config/nvim")); err != nil {
				return fail(err)
			}
		}
		if err = applyAppearance(root, home, m.theme, m.font); err != nil {
			return fail(err)
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
		return installMsg{lines: append(lines, "All selected components installed"), err: nil}
	}
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
		if _, e := os.Stat(dst); e == nil {
			if e = backup(dst); e != nil {
				return e
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
func applyAppearance(root, home string, theme, font int) error {
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
	prefs := fmt.Sprintf("theme=%s\nfont=%s\n", t, fontName)
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
