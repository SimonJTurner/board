package board

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiTickMsg struct{}
type tuiContextDoneMsg struct{}
type tuiEditorDoneMsg struct {
	err error
}

type watchTUIModel struct {
	ctx        context.Context
	store      *Store
	cfg        WatchConfig
	prev       BoardMeta
	board      BoardMeta
	selected   int
	showDetail bool
	detailDoc  IssueDoc
	detailErr  string
	statusLine string
	width      int
	height     int
}

func WatchTUI(ctx context.Context, store *Store, cfg WatchConfig) error {
	if cfg.Interval <= 0 {
		cfg.Interval = 2 * time.Second
	}
	if cfg.EnableMap == nil {
		cfg.EnableMap = DefaultEnabledEventTypes
	}

	initial, err := store.LoadBoard(cfg.Project)
	if err != nil {
		return err
	}

	m := watchTUIModel{
		ctx:      ctx,
		store:    store,
		cfg:      cfg,
		prev:     initial,
		board:    initial,
		selected: 0,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func (m watchTUIModel) Init() tea.Cmd {
	return tea.Batch(tuiTick(m.cfg.Interval), tuiWaitForContextDone(m.ctx))
}

func (m watchTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tuiContextDoneMsg:
		return m, tea.Quit
	case tuiTickMsg:
		curr, err := m.store.LoadBoard(m.cfg.Project)
		if err != nil {
			m.statusLine = fmt.Sprintf("watch error: %v", err)
			return m, tuiTick(m.cfg.Interval)
		}
		events := diffIssues(m.prev, curr, m.cfg.EnableMap)
		for _, ev := range events {
			emitEvent(ev, m.cfg.HookCmd, false)
			m.statusLine = formatEventLine(ev)
		}
		m.prev = curr
		m.board = curr
		m.normalizeSelection()
		if m.showDetail {
			m.loadDetail()
		}
		return m, tuiTick(m.cfg.Interval)
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if !m.showDetail && m.selected < len(m.board.Issues)-1 {
				m.selected++
			}
		case "k", "up":
			if !m.showDetail && m.selected > 0 {
				m.selected--
			}
		case "enter":
			if len(m.board.Issues) == 0 {
				return m, nil
			}
			if m.showDetail {
				return m, m.openEditorCmd()
			}
			m.showDetail = true
			m.loadDetail()
		case "b", "esc", "backspace":
			m.showDetail = false
		}
		return m, nil
	case tuiEditorDoneMsg:
		if msg.err != nil {
			m.statusLine = fmt.Sprintf("editor failed: %v", msg.err)
			return m, nil
		}
		// Reload board + selected issue immediately after editing so the UI
		// reflects any changes without waiting for the next tick.
		curr, err := m.store.LoadBoard(m.cfg.Project)
		if err != nil {
			m.statusLine = fmt.Sprintf("reload failed: %v", err)
			return m, nil
		}
		m.prev = curr
		m.board = curr
		m.normalizeSelection()
		m.loadDetail()
		m.statusLine = "saved issue changes"
		return m, nil
	default:
		return m, nil
	}
}

func (m watchTUIModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	footerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))

	lines := make([]string, 0, len(m.board.Issues)+10)
	lines = append(lines, titleStyle.Render("Board Watch: "+m.cfg.Project))
	lines = append(lines, helpStyle.Render("j/k move  Enter open details/edit  b back  q quit"))
	if m.statusLine != "" {
		lines = append(lines, helpStyle.Render("Last event: "+m.statusLine))
	}
	lines = append(lines, "")

	if m.showDetail {
		lines = append(lines, m.detailLines()...)
	} else {
		lines = append(lines, "Issues:")
		for i, issue := range m.board.Issues {
			cursor := " "
			if i == m.selected {
				cursor = ">"
			}
			line := fmt.Sprintf("%s #%d %s %s %s", cursor, issue.Number, statusIcon(issue.Status), issue.Status, issue.Title)
			if i == m.selected {
				line = selectedStyle.Render(line)
			}
			lines = append(lines, line)
		}
		if len(m.board.Issues) == 0 {
			lines = append(lines, helpStyle.Render("(no issues)"))
		}
	}

	todoCount := 0
	for _, issue := range m.board.Issues {
		if issue.Status == StatusTodo {
			todoCount++
		}
	}
	footer := footerStyle.Render(fmt.Sprintf("Todo left: %d", todoCount))

	if m.height > 1 && len(lines) < m.height-1 {
		padding := m.height - 1 - len(lines)
		for i := 0; i < padding; i++ {
			lines = append(lines, "")
		}
	}
	lines = append(lines, footer)
	return strings.Join(lines, "\n")
}

func (m *watchTUIModel) normalizeSelection() {
	if len(m.board.Issues) == 0 {
		m.selected = 0
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.board.Issues) {
		m.selected = len(m.board.Issues) - 1
	}
}

func (m *watchTUIModel) selectedIssueID() string {
	if len(m.board.Issues) == 0 || m.selected < 0 || m.selected >= len(m.board.Issues) {
		return ""
	}
	return m.board.Issues[m.selected].ID
}

func (m *watchTUIModel) loadDetail() {
	doc, _, err := m.store.GetIssue(m.cfg.Project, m.selectedIssueID())
	if err != nil {
		m.detailErr = err.Error()
		return
	}
	m.detailErr = ""
	m.detailDoc = doc
}

func (m watchTUIModel) openEditorCmd() tea.Cmd {
	issueID := m.selectedIssueID()
	if issueID == "" {
		return nil
	}
	p, err := m.store.GetIssueFilePath(m.cfg.Project, issueID)
	if err != nil {
		return func() tea.Msg { return tuiEditorDoneMsg{err: err} }
	}
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vim"
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		parts = []string{"vim"}
	}
	name := parts[0]
	args := append(parts[1:], p)
	cmd := exec.Command(name, args...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return tuiEditorDoneMsg{err: err}
	})
}

func (m watchTUIModel) detailLines() []string {
	header := lipgloss.NewStyle().Bold(true)
	lines := []string{header.Render("Issue Details")}
	if m.detailErr != "" {
		lines = append(lines, wrapPrefixed("error: ", m.detailErr, m.contentWidth())...)
		return lines
	}
	doc := m.detailDoc
	lines = append(lines, wrapPrefixed("ID: ", doc.ID, m.contentWidth())...)
	lines = append(lines, wrapPrefixed("Number: ", fmt.Sprintf("%d", doc.Number), m.contentWidth())...)
	lines = append(lines, wrapPrefixed("Title: ", doc.Title, m.contentWidth())...)
	lines = append(lines, wrapPrefixed("Status: ", fmt.Sprintf("%s %s", statusIcon(doc.Status), doc.Status), m.contentWidth())...)
	lines = append(lines, wrapPrefixed("Assignee: ", doc.Assignee, m.contentWidth())...)
	lines = append(lines, "")
	lines = append(lines, "Description:")
	for _, ln := range strings.Split(doc.Description, "\n") {
		if strings.TrimSpace(ln) == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapPrefixed("  ", ln, m.contentWidth())...)
	}
	return lines
}

func (m watchTUIModel) contentWidth() int {
	if m.width <= 0 {
		return 80
	}
	if m.width < 20 {
		return 20
	}
	return m.width - 2
}

var wordSplitter = regexp.MustCompile(`\S+\s*`)

func wrapPrefixed(prefix, text string, width int) []string {
	if width <= len(prefix)+1 {
		return []string{prefix + text}
	}
	avail := width - len(prefix)
	indent := strings.Repeat(" ", len(prefix))
	words := wordSplitter.FindAllString(text, -1)
	if len(words) == 0 {
		return []string{prefix}
	}

	lines := make([]string, 0, 4)
	current := ""
	for _, w := range words {
		trimmed := strings.TrimRight(w, " ")
		if len(trimmed) > avail {
			if current != "" {
				lines = append(lines, prefix+strings.TrimRight(current, " "))
				prefix = indent
				current = ""
			}
			for len(trimmed) > avail {
				lines = append(lines, prefix+trimmed[:avail])
				prefix = indent
				trimmed = trimmed[avail:]
			}
			if trimmed != "" {
				current = trimmed + " "
			}
			continue
		}
		if len(strings.TrimRight(current, " "))+len(trimmed) > avail {
			lines = append(lines, prefix+strings.TrimRight(current, " "))
			prefix = indent
			current = w
			continue
		}
		current += w
	}
	if strings.TrimSpace(current) != "" {
		lines = append(lines, prefix+strings.TrimRight(current, " "))
	}
	return lines
}

func tuiTick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg { return tuiTickMsg{} })
}

func tuiWaitForContextDone(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		<-ctx.Done()
		return tuiContextDoneMsg{}
	}
}
