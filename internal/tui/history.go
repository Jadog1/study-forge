// Package tui provides BubbleTea interactive TUI components for sfq.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Jadog1/study-forge/internal/session"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	accentColor  = lipgloss.Color("#7c6af0")
	correctColor = lipgloss.Color("#22d97a")
	wrongColor   = lipgloss.Color("#f05d7c")
	mutedColor   = lipgloss.Color("#8991b4")
	dimColor     = lipgloss.Color("#5a6080")
	bgColor      = lipgloss.Color("#0d0f1a")

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	correctStyle = lipgloss.NewStyle().Foreground(correctColor)
	wrongStyle   = lipgloss.NewStyle().Foreground(wrongColor)
	mutedStyle   = lipgloss.NewStyle().Foreground(mutedColor)
	dimStyle     = lipgloss.NewStyle().Foreground(dimColor)
	detailBox    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2e3354")).
			Background(bgColor).
			Padding(1, 2)
	helpStyle = lipgloss.NewStyle().Foreground(dimColor).Italic(true)
)

// ── View state ────────────────────────────────────────────────────────────────

type viewState int

const (
	viewList   viewState = iota
	viewDetail           // drill-down into a single session
)

// ── List item ─────────────────────────────────────────────────────────────────

// sessionItem adapts session.Session to the bubbles list.Item interface.
type sessionItem struct{ s session.Session }

func (i sessionItem) FilterValue() string {
	return i.s.QuizTitle + " " + strings.Join(i.s.Tags, " ") + " " + i.s.SessionID
}

func (i sessionItem) Title() string {
	score := mutedStyle.Render("in progress")
	if i.s.Score != nil {
		pct := i.s.Score.Pct
		col := correctStyle
		if pct < 50 {
			col = wrongStyle
		} else if pct < 70 {
			col = mutedStyle
		}
		score = col.Render(fmt.Sprintf("%d%%", pct)) +
			dimStyle.Render(fmt.Sprintf("  (%d/%d)", i.s.Score.Correct, i.s.TotalQuestions))
	}
	return i.s.QuizTitle + "  " + score
}

func (i sessionItem) Description() string {
	date := i.s.StartedAt.Local().Format("2006-01-02 15:04")
	tags := strings.Join(i.s.Tags, ", ")
	if tags == "" {
		tags = "no tags"
	}
	return dimStyle.Render(i.s.SessionID[:min(len(i.s.SessionID), 28)] + "  ·  " + date + "  ·  " + tags)
}

// ── Async messages ────────────────────────────────────────────────────────────

type answersLoadedMsg struct {
	sess    session.Session
	answers []session.Answer
}

type errMsg struct{ err error }

// ── Model ─────────────────────────────────────────────────────────────────────

// HistoryModel is the BubbleTea model for browsing session history.
type HistoryModel struct {
	view            viewState
	list            list.Model
	detail          session.Session
	answers         []session.Answer
	detailViewport  viewport.Model
	width           int
	height          int
	err             string
	RetakeSessionID string // set when user presses r in detail view
}

func (m HistoryModel) detailInnerHeight() int {
	if m.height-6 < 1 {
		return 1
	}
	return m.height - 6
}

func (m HistoryModel) detailBoxWidth() int {
	if m.width-4 < 10 {
		return 10
	}
	return m.width - 4
}

func (m HistoryModel) detailInnerWidth() int {
	// Rounded border + horizontal padding consume 6 columns.
	if m.detailBoxWidth()-6 < 1 {
		return 1
	}
	return m.detailBoxWidth() - 6
}

// syncDetailViewport rebuilds the viewport content and resizes it.
// Pass goTop=true when first entering the detail view so the header is always visible.
func (m HistoryModel) syncDetailViewport(goTop bool) HistoryModel {
	m.detailViewport.Width = m.detailInnerWidth()
	m.detailViewport.Height = m.detailInnerHeight()
	content := strings.Join(buildDetailLines(m.detail, m.answers), "\n")
	m.detailViewport.SetContent(content)
	if goTop {
		m.detailViewport.GotoTop()
	}
	return m
}

// NewHistory creates a HistoryModel populated with the given sessions.
func NewHistory(sessions []session.Session) HistoryModel {
	items := make([]list.Item, len(sessions))
	for i, s := range sessions {
		items[i] = sessionItem{s: s}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(accentColor).BorderLeftForeground(accentColor)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(mutedColor).BorderLeftForeground(accentColor)

	l := list.New(items, delegate, 0, 0)
	l.Title = "📚  StudyForge — Session History (d to delete)"
	l.Styles.Title = titleStyle.Padding(0, 1)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	vp := viewport.New(1, 1)

	return HistoryModel{list: l, view: viewList, detailViewport: vp}
}

func (m HistoryModel) Init() tea.Cmd { return nil }

// ── Update ────────────────────────────────────────────────────────────────────

func loadAnswersCmd(s session.Session) tea.Cmd {
	return func() tea.Msg {
		dir, err := session.SessionDirByID(s.SessionID)
		if err != nil {
			return errMsg{err}
		}
		answers, err := session.LoadAnswers(dir)
		if err != nil {
			return errMsg{err}
		}
		return answersLoadedMsg{sess: s, answers: answers}
	}
}

func (m HistoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(msg.Width, msg.Height)
		if m.view == viewDetail {
			// Resize only — preserve scroll position so the user stays where they are.
			m = m.syncDetailViewport(false)
		}
		return m, nil

	case answersLoadedMsg:
		m.detail = msg.sess
		m.answers = msg.answers
		m.view = viewDetail
		// Always show the header when first opening a session.
		m = m.syncDetailViewport(true)
		return m, nil

	case errMsg:
		m.err = msg.err.Error()
		return m, nil

	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.view {
		case viewList:
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "enter":
				if item, ok := m.list.SelectedItem().(sessionItem); ok {
					return m, loadAnswersCmd(item.s)
				}
			case "d", "delete":
				if item, ok := m.list.SelectedItem().(sessionItem); ok {
					if err := session.Delete(item.s.SessionID); err != nil {
						m.err = "delete failed: " + err.Error()
					} else {
						m.list.RemoveItem(m.list.Index())
					}
					return m, nil
				}
			}
		case viewDetail:
			switch msg.String() {
			case "q", "esc", "b":
				m.view = viewList
				m.answers = nil
				return m, nil
			case "r":
				m.RetakeSessionID = m.detail.SessionID
				return m, tea.Quit
			case "d", "delete":
				if err := session.Delete(m.detail.SessionID); err != nil {
					m.err = "delete failed: " + err.Error()
				} else {
					m.list.RemoveItem(m.list.Index())
					m.view = viewList
					m.answers = nil
				}
				return m, nil
			case "up", "k":
				m.detailViewport.LineUp(1)
				return m, nil
			case "down", "j":
				m.detailViewport.LineDown(1)
				return m, nil
			case "pgup", "u":
				m.detailViewport.HalfViewUp()
				return m, nil
			case "pgdown", "f":
				m.detailViewport.HalfViewDown()
				return m, nil
			}
		}
	}

	if m.view == viewList {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m HistoryModel) View() string {
	if m.err != "" {
		return wrongStyle.Render("Error: " + m.err)
	}
	switch m.view {
	case viewList:
		return m.list.View()
	case viewDetail:
		return m.renderDetail()
	}
	return ""
}

func (m HistoryModel) renderDetail() string {
	// View() must be side-effect-free: never call SetContent here.
	return detailBox.
		Width(m.detailBoxWidth()).
		Render(m.detailViewport.View())
}

func buildDetailLines(s session.Session, answers []session.Answer) []string {
	var lines []string

	// ── Header ──
	lines = append(lines, titleStyle.Render("📚  "+s.QuizTitle))

	date := s.StartedAt.Local().Format("2006-01-02  15:04:05")
	lines = append(lines, mutedStyle.Render("Session  "+s.SessionID))
	lines = append(lines, dimStyle.Render("Started  "+date))

	if s.EndedAt != nil {
		lines = append(lines, dimStyle.Render("Ended    "+s.EndedAt.Local().Format("2006-01-02  15:04:05")))
	}

	if s.Score != nil {
		c := correctStyle.Render(fmt.Sprintf("✓ %d correct", s.Score.Correct))
		w := wrongStyle.Render(fmt.Sprintf("✗ %d incorrect", s.Score.Incorrect))
		sk := dimStyle.Render(fmt.Sprintf("– %d skipped", s.Score.Skipped))
		pctCol := correctStyle
		if s.Score.Pct < 50 {
			pctCol = wrongStyle
		} else if s.Score.Pct < 70 {
			pctCol = mutedStyle
		}
		lines = append(lines, "Score    "+pctCol.Render(fmt.Sprintf("%d%%", s.Score.Pct))+"   "+c+"   "+w+"   "+sk)
	} else {
		lines = append(lines, mutedStyle.Render("Score    in progress"))
	}

	if len(s.Tags) > 0 {
		lines = append(lines, dimStyle.Render("Tags     "+strings.Join(s.Tags, ", ")))
	}

	lines = append(lines, strings.Repeat("─", 58))

	// ── Answers ──
	if len(answers) == 0 {
		lines = append(lines, dimStyle.Render("No answers recorded for this session."))
	} else {
		for i, a := range answers {
			mark := correctStyle.Render("✅")
			if !a.Correct {
				mark = wrongStyle.Render("❌")
			}
			title := truncRunes(a.QuestionTitle, 42)
			tags := ""
			if len(a.Tags) > 0 {
				tags = "  " + dimStyle.Render("["+strings.Join(a.Tags, ", ")+"]")
			}
			timing := dimStyle.Render(fmt.Sprintf("  %ds", a.TimeSpentSecs))
			lines = append(lines, fmt.Sprintf("  %2d.  %s  %s%s%s", i+1, mark, title, tags, timing))
		}
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("b/esc · back   ↑/↓ · scroll   r · retake   d · delete   q · quit"))

	return lines
}

// RunHistory launches the interactive history TUI.
// Returns the session ID to retake if the user pressed `r`, otherwise empty string.
func RunHistory(sessions []session.Session) (string, error) {
	m := NewHistory(sessions)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}
	if hm, ok := finalModel.(HistoryModel); ok {
		return hm.RetakeSessionID, nil
	}
	return "", nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
