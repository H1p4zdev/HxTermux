package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
)

type systemInfo struct {
	user, host, os, kernel, arch string
	shell, terminal, directory   string
	uptime, memory, cpu, device  string
	init, disk, brand            string
	packages                     string
	memoryPercent                float64
}

type systemRow struct{ icon, label, value string }

type bannerModel struct {
	info       systemInfo
	width      int
	height     int
	icons      bool
	frame      int
	splashRows int
	splashTick int
}

type logoFrameMsg struct{}
type finishBannerMsg struct{}

const (
	splashFrameInterval = 8 * time.Millisecond
	splashDuration      = 3 * time.Second
)

var (
	cyan    = lipgloss.Color("#52E0D0")
	purple  = lipgloss.Color("#C792EA")
	muted   = lipgloss.Color("#8792A8")
	text    = lipgloss.Color("#E8EDF7")
	panelBG = lipgloss.Color("#111827")
	border  = lipgloss.Color("#52617A")
)

func main() {
	noIcons := flag.Bool("no-icons", false, "hide Nerd Font icons")
	themeMenu := flag.Bool("theme", false, "open the full-screen customization menu")
	themePreset := flag.String("theme-preset", "", "use and remember a theme preset")
	listThemes := flag.Bool("list-themes", false, "list available themes")
	flag.Parse()
	if *listThemes {
		fmt.Println("Available themes:")
		for _, theme := range themeCatalog {
			fmt.Printf("  %-18s %s\n", theme.ID, theme.Name)
		}
		return
	}
	prefs := loadThemePreferences()
	selectedTheme := prefs.Selected
	if *themePreset != "" {
		selectedTheme = *themePreset
	}
	activeFontStyle = prefs.FontStyle
	if err := applyTheme(selectedTheme); err != nil {
		fmt.Fprintln(os.Stderr, "HypexFetch:", err)
		os.Exit(2)
	}
	if *themeMenu {
		index := 0
		for i, theme := range themeCatalog {
			if theme.ID == activeTheme.ID {
				index = i
				break
			}
		}
		fontIndex := 0
		for i, font := range fontCatalog {
			if font.ID == activeFontStyle {
				fontIndex = i
				break
			}
		}
		editor := themeEditorModel{selectedTheme: index, selectedFont: fontIndex}
		p := tea.NewProgram(editor, tea.WithColorProfile(colorprofile.TrueColor))
		if _, err := p.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "HypexFetch theme studio:", err)
			os.Exit(1)
		}
		return
	}
	if *themePreset != "" {
		if err := saveTheme(activeTheme.ID, activeFontStyle); err != nil {
			fmt.Fprintln(os.Stderr, "HypexFetch: couldn't remember theme:", err)
		}
	}
	width, height := terminalSize()
	m := bannerModel{info: collectInfo(), width: width, height: height, icons: !*noIcons}
	p := tea.NewProgram(m, tea.WithColorProfile(colorprofile.TrueColor))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "HypexFetch:", err)
		os.Exit(1)
	}
}

func (m bannerModel) Init() tea.Cmd {
	return tea.Tick(splashFrameInterval, func(time.Time) tea.Msg { return logoFrameMsg{} })
}

func (m bannerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case logoFrameMsg:
		lines := strings.Split(strings.TrimRight(renderBanner(m.info, m.width, m.height, m.icons, m.frame), "\n"), "\n")
		totalTicks := int(splashDuration / splashFrameInterval)
		if m.splashTick < totalTicks {
			m.splashTick++
			m.splashRows = splashEaseOut(m.splashTick, totalTicks, len(lines))
			if m.frame < 3 {
				m.frame++
			}
			return m, tea.Tick(splashFrameInterval, func(time.Time) tea.Msg { return logoFrameMsg{} })
		}
		return m, tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg { return finishBannerMsg{} })
	case finishBannerMsg:
		return m, tea.Quit
	}
	return m, nil
}

// splashEaseOut eases the reveal across the full splash duration.
func splashEaseOut(tick, duration, totalRows int) int {
	if totalRows <= 1 {
		return totalRows
	}
	t := min(1, float64(tick)/float64(max(1, duration)))
	eased := t * t * (3 - 2*t)
	return min(totalRows, max(1, int(float64(totalRows)*eased+0.5)))
}

func (m bannerModel) View() tea.View {
	content := renderBanner(m.info, m.width, m.height, m.icons, m.frame)
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	fade := min(1, float64(m.splashRows)/float64(max(1, len(lines))))
	content = fadeANSI(content, fade)
	if m.splashTick < int(splashDuration/splashFrameInterval) {
		content = rainbowANSI(content, m.splashTick/6, fade)
	}
	lines = strings.Split(strings.TrimRight(content, "\n"), "\n")
	if m.splashRows < len(lines) {
		lines = lines[:m.splashRows]
	}
	view := tea.NewView(strings.Join(lines, "\n"))
	view.BackgroundColor = themeBackground
	return view
}

func fadeANSI(input string, progress float64) string {
	progress = max(0, min(1, progress))
	if progress >= 1 {
		return input
	}
	bgR, bgG, bgB := themeRGB(activeTheme.Background)
	var out strings.Builder
	for len(input) > 0 {
		start := strings.Index(input, "\x1b[")
		if start < 0 {
			out.WriteString(input)
			break
		}
		out.WriteString(input[:start])
		input = input[start:]
		end := strings.IndexByte(input, 'm')
		if end < 0 {
			out.WriteString(input)
			break
		}
		params := strings.Split(input[2:end], ";")
		var styled []string
		for i := 0; i < len(params); i++ {
			switch params[i] {
			case "38", "48":
				if i+4 < len(params) && params[i+1] == "2" {
					r, errR := strconv.Atoi(params[i+2])
					g, errG := strconv.Atoi(params[i+3])
					b, errB := strconv.Atoi(params[i+4])
					if errR == nil && errG == nil && errB == nil {
						styled = append(styled, params[i], "2", blendANSI(r, bgR, progress), blendANSI(g, bgG, progress), blendANSI(b, bgB, progress))
						i += 4
						continue
					}
				}
				styled = append(styled, params[i])
			case "37", "97", "47":
				channel := 170
				if params[i] == "97" || params[i] == "47" {
					channel = 255
				}
				mode := "38"
				if params[i] == "47" {
					mode = "48"
				}
				styled = append(styled, mode, "2", blendANSI(channel, bgR, progress), blendANSI(channel, bgG, progress), blendANSI(channel, bgB, progress))
			default:
				styled = append(styled, params[i])
			}
		}
		out.WriteString("\x1b[" + strings.Join(styled, ";") + "m")
		input = input[end+1:]
	}
	return out.String()
}

func blendANSI(source, background int, progress float64) string {
	value := float64(background) + (float64(source)-float64(background))*progress
	return strconv.Itoa(int(value + 0.5))
}

func themeRGB(hex string) (int, int, int) {
	value, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil {
		return 26, 27, 38
	}
	return int(value >> 16), int((value >> 8) & 0xff), int(value & 0xff)
}

func rainbowANSI(input string, phase int, intensity float64) string {
	palette := [][3]int{
		{247, 118, 142}, {255, 158, 100}, {224, 175, 104}, {158, 206, 106},
		{42, 195, 222}, {122, 162, 247}, {199, 146, 234},
	}
	var out strings.Builder
	colorIndex := phase
	bgR, bgG, bgB := themeRGB(activeTheme.Background)
	for len(input) > 0 {
		start := strings.Index(input, "\x1b[")
		if start < 0 {
			out.WriteString(input)
			break
		}
		out.WriteString(input[:start])
		input = input[start:]
		end := strings.IndexByte(input, 'm')
		if end < 0 {
			out.WriteString(input)
			break
		}
		params := strings.Split(input[2:end], ";")
		var styled []string
		for i := 0; i < len(params); i++ {
			if params[i] == "38" && i+4 < len(params) && params[i+1] == "2" {
				chosen := palette[((colorIndex%len(palette))+len(palette))%len(palette)]
				styled = append(styled, "38", "2",
					blendANSI(chosen[0], bgR, intensity),
					blendANSI(chosen[1], bgG, intensity),
					blendANSI(chosen[2], bgB, intensity))
				i += 4
				colorIndex++
				continue
			}
			if params[i] == "37" || params[i] == "97" {
				chosen := palette[((colorIndex%len(palette))+len(palette))%len(palette)]
				styled = append(styled, "38", "2",
					blendANSI(chosen[0], bgR, intensity),
					blendANSI(chosen[1], bgG, intensity),
					blendANSI(chosen[2], bgB, intensity))
				colorIndex++
				continue
			}
			styled = append(styled, params[i])
		}
		out.WriteString("\x1b[" + strings.Join(styled, ";") + "m")
		input = input[end+1:]
	}
	return out.String()
}

func terminalSize() (int, int) {
	if width, height, err := term.GetSize(os.Stdout.Fd()); err == nil && width > 0 && height > 0 {
		return width, height
	}
	width, _ := strconv.Atoi(os.Getenv("COLUMNS"))
	height, _ := strconv.Atoi(os.Getenv("LINES"))
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return width, height
}

func renderBanner(info systemInfo, width, height int, icons bool, frame int) string {
	width = max(1, width)
	height = max(1, height)
	outerWidth := min(max(1, width-2), 86)
	innerWidth := max(1, outerWidth-2)
	contentRows := max(1, height-7)
	var inside string

	if innerWidth >= 34 {
		const padding, gap = 2, 3
		available := innerWidth - 2*padding - gap
		available = max(2, available)
		leftWidth := available * 2 / 3
		rightWidth := available - leftWidth
		specs := renderSpecs(info, icons, leftWidth, contentRows)
		specLines := strings.Split(specs, "\n")
		brandLines := strings.Split(renderBrandBox(rightWidth, height, frame), "\n")
		childRows := max(len(specLines), len(brandLines))
		if childRows > contentRows {
			childRows = contentRows
		}
		specLines = fitLines(specLines, childRows)
		brandLines = centerLines(brandLines, childRows)
		rows := make([]string, 0, childRows+7)
		rows = append(rows, strings.Repeat(" ", innerWidth))
		for i := 0; i < childRows; i++ {
			left := padRight(specLines[i], leftWidth)
			right := lipgloss.NewStyle().Width(rightWidth).MaxWidth(rightWidth).Align(lipgloss.Center).Render(brandLines[i])
			rows = append(rows, strings.Repeat(" ", padding)+left+strings.Repeat(" ", gap)+right+strings.Repeat(" ", padding))
		}
		rows = append(rows, strings.Repeat(" ", innerWidth))
		inside = strings.Join(rows, "\n")
	} else {
		// Stack narrow panels inside the same padded parent frame.
		panelWidth := max(1, innerWidth-4)
		brand := renderBrandBox(panelWidth, height, frame)
		logoRows := lipgloss.Height(brand)
		specRows := max(1, height-9-logoRows)
		specs := renderSpecs(info, icons, panelWidth, specRows)
		brandLines := strings.Split(brand, "\n")
		rows := []string{strings.Repeat(" ", innerWidth)}
		for _, line := range strings.Split(specs, "\n") {
			rows = append(rows, "  "+padRight(line, panelWidth)+strings.Repeat(" ", innerWidth-panelWidth-2))
		}
		rows = append(rows, strings.Repeat(" ", innerWidth))
		for _, line := range brandLines {
			logo := lipgloss.NewStyle().Width(panelWidth).MaxWidth(panelWidth).Align(lipgloss.Center).Render(line)
			rows = append(rows, "  "+logo+strings.Repeat(" ", innerWidth-panelWidth-2))
		}
		rows = append(rows, strings.Repeat(" ", innerWidth))
		inside = strings.Join(rows, "\n")
	}

	boxed := outlineBox(inside, outerWidth, false)
	boxLines := strings.Split(boxed, "\n")
	badgeLines := strings.Split(renderTitleBox(innerWidth), "\n")
	framedBadge := make([]string, 0, len(badgeLines))
	for _, line := range badgeLines {
		framedBadge = append(framedBadge, lipgloss.NewStyle().Foreground(border).Render("│")+line+lipgloss.NewStyle().Foreground(border).Render("│"))
	}
	boxLines = append(boxLines[:1], append(framedBadge, boxLines[1:]...)...)
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, strings.Join(boxLines, "\n")) + "\n \n "
}

func renderTitleBox(width int) string {
	font := selectedTitleFont(activeFontStyle)
	nameWidth := lipgloss.Width(font.Hypex + font.Fetch)
	innerWidth := min(max(nameWidth+2, nameWidth*2+2), max(1, width-2))
	innerWidth = max(nameWidth, innerWidth-2)
	line := lipgloss.NewStyle().Foreground(cyan).Bold(true).Background(themeBackground)
	title := lipgloss.NewStyle().Foreground(cyan).Bold(true).Render(font.Hypex) +
		lipgloss.NewStyle().Foreground(purple).Bold(true).Render(font.Fetch)
	pad := max(0, (innerWidth-nameWidth)/2)
	rightPad := max(0, innerWidth-nameWidth-pad)
	badge := line.Render("┌"+strings.Repeat("─", pad)+" ") + title + line.Render(" "+strings.Repeat("─", rightPad)+"┐")
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, badge)
}

func padRight(line string, width int) string {
	padding := max(0, width-lipgloss.Width(line))
	return line + strings.Repeat(" ", padding)
}

func fitLines(lines []string, rows int) []string {
	if rows <= 0 {
		return nil
	}
	if len(lines) > rows {
		lines = lines[:rows]
	}
	for len(lines) < rows {
		lines = append(lines, " ")
	}
	return lines
}

func centerLines(lines []string, rows int) []string {
	if len(lines) >= rows {
		return fitLines(lines, rows)
	}
	pad := rows - len(lines)
	return append(append(make([]string, pad/2), lines...), make([]string, pad-pad/2)...)
}

func infoRows(info systemInfo) []systemRow {
	packages := strings.TrimSuffix(info.packages, " packages")
	memory := strings.ReplaceAll(info.memory, " MiB", " MB")
	rows := []systemRow{
		{"", "phone", strings.TrimSpace(info.brand + " " + info.device)},
		{"", "os", info.os + " " + info.arch},
		{"", "ker", info.kernel},
		{"", "pkgs", packages},
		{"", "sh", info.shell},
		{"", "ram", memory},
		{"", "init", info.init},
		{"", "up", info.uptime},
		{"", "disk", info.disk},
	}
	if info.brand == "" {
		rows = rows[1:]
	}
	return rows
}

func renderFetchRows(rows []systemRow, width int) string {
	colors := []color.Color{lipgloss.Color("#F7768E"), purple, lipgloss.Color("#9ECE6A"), cyan, lipgloss.Color("#7AA2F7"), lipgloss.Color("#E0AF68")}
	lines := make([]string, 0, len(rows))
	for i, row := range rows {
		label := lipgloss.NewStyle().Foreground(colors[i%len(colors)]).Bold(true).Width(5).Render(row.label)
		value := lipgloss.NewStyle().Foreground(text).Width(max(1, width-6)).MaxWidth(max(1, width-6)).Render(shorten(row.value, max(1, width-6)))
		lines = append(lines, label+" "+value)
	}
	return strings.Join(lines, "\n")
}

func renderMascot(width, rows, frame int) string {
	art := strings.Split(hypexCharacter(frame), "\n")
	if width < 15 {
		art = strings.Split(mediumHypexCharacter(frame), "\n")
	}
	if rows < len(art) {
		art = art[:rows]
	}
	for i := range art {
		art[i] = lipgloss.NewStyle().Width(width).MaxWidth(width).Align(lipgloss.Center).Render(art[i])
	}
	return strings.Join(art, "\n")
}

func renderSpecs(info systemInfo, icons bool, width, maxRows int) string {
	width = max(8, width)
	density := contentDensity(width, maxRows)
	if density >= 1 {
		info.device = shorten(info.device, max(9, width-8))
		info.kernel = shorten(info.kernel, max(9, width-8))
		info.packages = strings.TrimSuffix(info.packages, " packages") + " pkgs"
		info.memory = strings.ReplaceAll(info.memory, " MiB", "M")
		info.terminal = shorten(info.terminal, max(9, width-8))
	}
	if density >= 2 {
		icons = false
		if fields := strings.Fields(info.cpu); len(fields) > 1 {
			info.cpu = fields[len(fields)-1]
		}
	}
	deviceRows := []systemRow{
		{"󰍻", "OS", info.os}, {"󰏲", "Device", info.device}, {"󰒔", "Kernel", info.kernel},
		{"󰍛", "CPU", info.cpu}, {"󰘚", "Arch", info.arch}, {"󰏖", "Packages", info.packages},
	}
	sessionRows := []systemRow{
		{"󰥔", "Uptime", info.uptime}, {"󰆍", "Shell", info.shell}, {"󰆍", "Terminal", info.terminal},
	}
	iconWidth, labelWidth := 0, 10
	if icons {
		iconWidth = 3
		if width < 28 {
			iconWidth = 2
		}
	}
	if density >= 2 {
		labelWidth = 4
	} else if density >= 1 {
		labelWidth = 5
	}
	labelWidth = max(2, labelWidth*90/100)
	barIndent := iconWidth + labelWidth + 1
	barWidth := max(3, (width-barIndent)*2/3)
	bar := progress.New(progress.WithDefaultBlend(), progress.WithWidth(barWidth), progress.WithoutPercentage())
	heading := "SYSTEM SPECS"
	if width < 14 {
		heading = "SPECS"
	}
	headingLine := lipgloss.NewStyle().Foreground(cyan).Bold(true).Render(heading)
	identityWidth := width - lipgloss.Width(heading) - lipgloss.Width(" · ")
	if identityWidth >= 3 {
		identity := shorten(strings.TrimSpace(info.user+"@"+info.host), identityWidth)
		headingLine += lipgloss.NewStyle().Foreground(muted).Render(" · ") + lipgloss.NewStyle().Foreground(purple).Bold(true).Render(identity)
	}
	lines := []string{lipgloss.NewStyle().Width(width).MaxWidth(width).Render(headingLine)}
	appendIfFits := func(line string) bool {
		candidate := append(append([]string(nil), lines...), line)
		if visualHeight(strings.Join(candidate, "\n"), width) > maxRows {
			return false
		}
		lines = candidate
		return true
	}
	for _, row := range deviceRows {
		appendIfFits(renderRow(row, icons, width, density))
	}
	if density >= 2 {
		return strings.Join(lines, "\n")
	}
	if appendIfFits("") && appendIfFits(lipgloss.NewStyle().Foreground(cyan).Bold(true).Width(width).MaxWidth(width).Render("SESSION & RESOURCES")) {
		for _, row := range sessionRows {
			appendIfFits(renderRow(row, icons, width, density))
		}
		appendIfFits(renderRow(systemRow{"󰍛", "Memory", info.memory}, icons, width, density))
		appendIfFits(strings.Repeat(" ", barIndent) + bar.ViewAs(info.memoryPercent))
	}
	return strings.Join(lines, "\n")
}

func contentDensity(width, rows int) int {
	if width < 24 || rows < 18 {
		return 2 // short values, no icon column, system essentials only
	}
	if width < 34 || rows < 24 {
		return 1 // compact labels and values
	}
	return 0
}

func visualHeight(content string, width int) int {
	width = max(1, width)
	height := 0
	for _, line := range strings.Split(content, "\n") {
		lineWidth := lipgloss.Width(line)
		height += max(1, (lineWidth+width-1)/width)
	}
	return height
}

func outlineBox(content string, outerWidth int, centered bool) string {
	innerWidth := max(1, outerWidth-2)
	contentLines := strings.Split(content, "\n")
	var rows []string
	for _, line := range contentLines {
		wrapped := line
		if lipgloss.Width(wrapped) > innerWidth {
			wrapped = lipgloss.NewStyle().Width(innerWidth).MaxWidth(innerWidth).Render(wrapped)
		}
		rows = append(rows, strings.Split(wrapped, "\n")...)
	}
	lineStyle := lipgloss.NewStyle().Foreground(border)
	topStyle := lipgloss.NewStyle().Foreground(cyan).Bold(true)
	shadowStyle := lipgloss.NewStyle().Foreground(themeFill)
	var out strings.Builder
	out.WriteString(topStyle.Render("┏" + strings.Repeat("━", innerWidth) + "┓"))
	if innerWidth >= 3 {
		out.WriteByte('\n')
		out.WriteString(lineStyle.Render("│ ") + shadowStyle.Render(strings.Repeat("█", innerWidth-2)) + lineStyle.Render(" │"))
	}
	for _, text := range rows {
		out.WriteByte('\n')
		textWidth := min(innerWidth, lipgloss.Width(text))
		leftPad := 0
		if centered {
			leftPad = max(0, (innerWidth-textWidth)/2)
		}
		rightPad := max(0, innerWidth-leftPad-textWidth)
		out.WriteString(lineStyle.Render("│"))
		out.WriteString(strings.Repeat(" ", leftPad))
		out.WriteString(text)
		out.WriteString(strings.Repeat(" ", rightPad))
		out.WriteString(lineStyle.Render("│"))
	}
	if innerWidth >= 3 {
		out.WriteByte('\n')
		out.WriteString(lineStyle.Render("│ ") + shadowStyle.Render(strings.Repeat("█", innerWidth-2)) + lineStyle.Render(" │"))
	}
	out.WriteByte('\n')
	out.WriteString(topStyle.Render("┗" + strings.Repeat("━", innerWidth) + "┛"))
	return out.String()
}

func renderBrandBox(width, terminalRows, frame int) string {
	_ = terminalRows
	_ = frame
	shape := []string{
		" ################  ",
		" ##            .   ",
		" ##  ##        ##  ",
		" ##  ##        ##  ",
		" ##            ##  ",
		" ##            .   ",
		" ################  ",
	}
	artWidth := min(19, max(1, width))
	lines := make([]string, len(shape))
	for y, source := range shape {
		sourceRunes := []rune(source)
		var line strings.Builder
		for x := 0; x < artWidth; x++ {
			sourceX := x * len(sourceRunes) / artWidth
			switch sourceRunes[sourceX] {
			case '#':
				line.WriteString("\x1b[47m \x1b[0m")
			case '.':
				line.WriteString("\x1b[0;37m\x1b[47m.\x1b[0m")
			default:
				line.WriteByte(' ')
			}
		}
		lines[y] = line.String()
	}
	return strings.Join(lines, "\n")
}

func mediumHypexCharacter(frame int) string {
	lines := []string{"   ▄█▄   ", " ▄█▀ ▀█▄ ", " █ ▪ ▪ █ ", " ▀█▄▄▄█▀ ", "   ▀█▀   "}
	if frame == 1 {
		lines = []string{"   ░▒░   ", " ░▒███▒░ ", " ░█   █░ ", "  ░███░  ", "   ░▓░   "}
	}
	var out strings.Builder
	for i, line := range lines {
		color := cyan
		if i >= 3 {
			color = purple
		}
		out.WriteString(lipgloss.NewStyle().Foreground(color).Bold(true).Render(line))
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func compactHypexCharacter(frame int) string {
	lines := []string{"  ▄█▄  ", " █ ▪ █ ", "  ▀█▀  "}
	if frame == 1 {
		lines = []string{"  ░▒░  ", " ░███░ ", "  ░▓░  "}
	}
	var out strings.Builder
	for i, line := range lines {
		color := cyan
		if i == 2 {
			color = purple
		}
		out.WriteString(lipgloss.NewStyle().Foreground(color).Bold(true).Render(line))
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func renderCompactTitle(width int) string {
	font := selectedTitleFont(activeFontStyle)
	first, second := font.Hypex, font.Fetch
	if lipgloss.Width(first+second) > width {
		first, second = "HYPEX", "FETCH"
	}
	name := lipgloss.NewStyle().Foreground(cyan).Bold(true).Render(first) +
		lipgloss.NewStyle().Foreground(purple).Bold(true).Render(second)
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, name)
}

func hypexCharacter(frame int) string {
	if frame == 1 {
		art := `       ░▒▓▒░
     ░▒▓███▓▒░
    ░▓▒░   ░▒▓░
    ░▓░ ░ ░ ░▓░
    ░▓▒░ ▄ ░▒▓░
     ░▒▓███▓▒░
       ░▓ ▓░`
		return lipgloss.NewStyle().Foreground(muted).Bold(true).Render(art)
	}
	lines := []string{
		"       ▄█████▄",
		"     ▄█▀     ▀█▄",
		"    █  ▪     ▪  █",
		"    █    ▄    █",
		"    ▀█▄▄█████▄▄█▀",
		"       ▄█ H █▄",
		"       ▀█▄▄▄█▀",
	}
	var b strings.Builder
	for i, line := range lines {
		color := cyan
		if i >= 5 {
			color = purple
		}
		if frame < 3 && i == 2 {
			line = "    █  ░     ░  █"
		}
		b.WriteString(lipgloss.NewStyle().Foreground(color).Bold(true).Render(line))
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func renderRow(row systemRow, icons bool, width, density int) string {
	iconWidth, labelWidth := 0, 10
	icon := ""
	if icons {
		iconWidth = 3
		if width < 28 {
			iconWidth = 2
		}
		icon = lipgloss.NewStyle().Foreground(iconColor(row.icon)).Width(iconWidth).Render(row.icon)
	}
	if density >= 2 {
		labelWidth = 4
		row.label = compactLabel(row.label)
	} else if density >= 1 {
		labelWidth = 5
		row.label = compactLabel(row.label)
	}
	labelWidth = max(2, labelWidth*90/100)
	labelText := shorten(row.label, labelWidth)
	label := lipgloss.NewStyle().Foreground(purple).Bold(true).Width(labelWidth).MaxWidth(labelWidth).Render(labelText)
	valueWidth := max(3, width-iconWidth-labelWidth-1)
	valueWidth = max(3, valueWidth*90/100)
	value := lipgloss.NewStyle().Foreground(text).Width(valueWidth).MaxWidth(valueWidth).
		Render(shorten(row.value, valueWidth))
	return icon + label + " " + value
}

func iconColor(icon string) color.Color {
	hash := 0
	for _, r := range icon {
		hash = (hash*31 + int(r)) % len(iconPalette)
	}
	return iconPalette[hash]
}

func shorten(value string, maxWidth int) string {
	if maxWidth < 2 || lipgloss.Width(value) <= maxWidth {
		return value
	}
	runes := []rune(value)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > maxWidth {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func compactLabel(label string) string {
	switch label {
	case "Device":
		return "Dev"
	case "Kernel":
		return "Kern"
	case "Packages":
		return "Pkgs"
	case "Uptime":
		return "Up"
	case "Shell":
		return "Sh"
	case "Terminal":
		return "Term"
	case "Memory":
		return "Mem"
	default:
		return label
	}
}

func colorPalette(width int) string {
	blockWidth := 3
	if width < 54 {
		blockWidth = 2
	}
	if width < 36 {
		blockWidth = 1
	}
	block := strings.Repeat(" ", blockWidth)
	var b strings.Builder
	for _, color := range []string{"0", "1", "2", "3", "4", "5", "6", "7"} {
		b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(color)).Render(block))
	}
	b.WriteByte(' ')
	for _, color := range []string{"8", "9", "10", "11", "12", "13", "14", "15"} {
		b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(color)).Render(block))
	}
	return b.String()
}
