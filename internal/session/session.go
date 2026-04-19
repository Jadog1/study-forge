// Package session manages persistent quiz session state for sfq.
// Sessions are stored as flat files under ~/.sfq/sessions/<session-id>/:
//
//	meta.json    — session metadata (title, score, timestamps)
//	answers.jsonl — one JSON line per submitted answer (append-only)
package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Jadog1/study-forge/internal/parser"
)

// ── Data Types ────────────────────────────────────────────────────────────────

// Score holds the final quiz result breakdown.
type Score struct {
	Correct   int `json:"correct"`
	Partial   int `json:"partial"` // count of partially-correct answers (0 < credit < 1)
	Incorrect int `json:"incorrect"`
	Skipped   int `json:"skipped"`
	Pct       int `json:"pct"` // weighted integer percentage (partial credits count fractionally)
}

// Session represents the metadata for a single quiz session.
type Session struct {
	SessionID      string     `json:"session_id"`
	QuizFile       string     `json:"quiz_file"`
	QuizTitle      string     `json:"quiz_title"`
	Tags           []string   `json:"tags"`
	StartedAt      time.Time  `json:"started_at"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	TotalQuestions int        `json:"total_questions"`
	Score          *Score     `json:"score,omitempty"`
}

// Answer represents a single submitted answer within a session.
type Answer struct {
	QuestionID    string    `json:"question_id"`
	QuestionTitle string    `json:"question_title"`
	Tags          []string  `json:"tags"`
	Type          string    `json:"type"`
	Correct       bool      `json:"correct"`
	PartialCredit float64   `json:"partial_credit,omitempty"` // 0.0–1.0; present on question types that support partial scoring
	SubmittedAt   time.Time `json:"submitted_at"`
	TimeSpentSecs int       `json:"time_spent_s"`
}

// ListSessionsFilter controls filtering and pagination for ListSessions.
type ListSessionsFilter struct {
	Tags   []string  // if non-empty, session must have at least one matching tag
	Since  time.Time // zero = no lower bound
	Until  time.Time // zero = no upper bound
	Limit  int       // 0 = return all
	Offset int       // skip this many results (after filtering, before limit)
}

// ── Paths ─────────────────────────────────────────────────────────────────────

func sessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".sfq", "sessions"), nil
}

func sessionDir(sessionID string) (string, error) {
	base, err := sessionsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, sessionID), nil
}

// ── Session Control ──────────────────────────────────────────────────────────

// Delete completely removes a session directory from disk.
func Delete(sessionID string) error {
	dir, err := sessionDir(sessionID)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// Create initialises a new session on disk and returns its metadata.
// The session directory is created at ~/.sfq/sessions/<session-id>/.
func Create(qf *parser.QuizFile, sfqPath string) (*Session, error) {
	id := newSessionID(qf.Title)

	dir, err := sessionDir(id)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create session directory: %w", err)
	}

	// Collect the union of all question tags in the quiz.
	tagSet := map[string]struct{}{}
	for _, q := range qf.Questions {
		for _, t := range q.Tags {
			tagSet[t] = struct{}{}
		}
	}
	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)

	absPath, err := filepath.Abs(sfqPath)
	if err != nil {
		absPath = sfqPath
	}

	s := &Session{
		SessionID:      id,
		QuizFile:       absPath,
		QuizTitle:      qf.Title,
		Tags:           tags,
		StartedAt:      time.Now().UTC(),
		TotalQuestions: len(qf.Questions),
	}

	if err := writeMetaJSON(dir, s); err != nil {
		return nil, err
	}
	return s, nil
}

// newSessionID produces a human-readable, filesystem-safe session ID.
// Format: YYYYMMDD-HHMMSS-<slug>-<4 hex chars>
func newSessionID(title string) string {
	ts := time.Now().UTC().Format("20060102-150405")
	slug := slugify(title)
	suffix := fmt.Sprintf("%04x", rand.Intn(0xffff))
	if slug == "" {
		return ts + "-" + suffix
	}
	return ts + "-" + slug + "-" + suffix
}

// slugify converts a title to a lowercase, hyphen-separated ASCII slug,
// capped at 24 characters to keep directory names manageable.
var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 24 {
		s = s[:24]
		s = strings.TrimRight(s, "-")
	}
	return s
}

// ── Answer Recording ──────────────────────────────────────────────────────────

// AppendAnswer appends a single answer record to answers.jsonl in the session directory.
// This is safe to call concurrently only if callers serialize writes; the server
// processes /answer requests sequentially via its own mutex.
func AppendAnswer(dir string, a Answer) error {
	path := filepath.Join(dir, "answers.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open answers file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("cannot marshal answer: %w", err)
	}
	_, err = fmt.Fprintln(f, string(data))
	return err
}

// ── Session Finalisation ──────────────────────────────────────────────────────

// ComputeScore derives a Score from stored Answer records.
// Partial credit (0 < PartialCredit < 1) is weighted in the Pct calculation.
func ComputeScore(answers []Answer, total int) Score {
	var (
		correct        int
		partial        int
		incorrect      int
		weightedPoints float64
	)
	for _, a := range answers {
		if a.Correct {
			correct++
			weightedPoints += 1.0
		} else if a.PartialCredit > 0 {
			partial++
			weightedPoints += a.PartialCredit
		} else {
			incorrect++
		}
	}
	skipped := total - correct - partial - incorrect
	if skipped < 0 {
		skipped = 0
	}
	pct := 0
	if total > 0 {
		pct = int(math.Round(weightedPoints / float64(total) * 100))
	}
	return Score{
		Correct:   correct,
		Partial:   partial,
		Incorrect: incorrect,
		Skipped:   skipped,
		Pct:       pct,
	}
}

// Finish updates meta.json with the final score and ended_at timestamp.
// The score is derived from the stored answers.jsonl, so the caller does not
// need to supply it. It is safe to call multiple times (idempotent).
func Finish(dir string) error {
	s, err := LoadMeta(dir)
	if err != nil {
		return err
	}
	answers, _ := LoadAnswers(dir)
	score := ComputeScore(answers, s.TotalQuestions)
	now := time.Now().UTC()
	s.EndedAt = &now
	s.Score = &score
	return writeMetaJSON(dir, s)
}

// ── Loading ───────────────────────────────────────────────────────────────────

// LoadMeta reads the meta.json for the session at dir.
func LoadMeta(dir string) (*Session, error) {
	path := filepath.Join(dir, "meta.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read meta.json: %w", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("corrupt meta.json: %w", err)
	}
	return &s, nil
}

// LoadAnswers reads all answer lines from answers.jsonl in dir.
func LoadAnswers(dir string) ([]Answer, error) {
	path := filepath.Join(dir, "answers.jsonl")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil // no answers yet
	}
	if err != nil {
		return nil, fmt.Errorf("cannot open answers.jsonl: %w", err)
	}
	defer f.Close()

	var answers []Answer
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var a Answer
		if err := json.Unmarshal([]byte(line), &a); err != nil {
			return nil, fmt.Errorf("corrupt answers.jsonl line: %w", err)
		}
		answers = append(answers, a)
	}
	return answers, scanner.Err()
}

// ── Listing ───────────────────────────────────────────────────────────────────

// ListSessions returns sessions matching the given filter, sorted newest-first.
// A zero ListSessionsFilter returns all sessions.
func ListSessions(opts ListSessionsFilter) ([]Session, error) {
	base, err := sessionsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read sessions directory: %w", err)
	}

	var sessions []Session
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(base, e.Name())
		s, err := LoadMeta(dir)
		if err != nil {
			continue // skip corrupt/incomplete sessions silently
		}

		// ── Tag filter ──
		if len(opts.Tags) > 0 && !sessionHasAnyTag(s, opts.Tags) {
			continue
		}
		// ── Date filters ──
		if !opts.Since.IsZero() && s.StartedAt.Before(opts.Since) {
			continue
		}
		if !opts.Until.IsZero() && s.StartedAt.After(opts.Until) {
			continue
		}

		sessions = append(sessions, *s)
	}

	// Sort newest-first.
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})

	// ── Pagination ──
	if opts.Offset > 0 {
		if opts.Offset >= len(sessions) {
			return nil, nil
		}
		sessions = sessions[opts.Offset:]
	}
	if opts.Limit > 0 && len(sessions) > opts.Limit {
		sessions = sessions[:opts.Limit]
	}

	return sessions, nil
}

// SessionDirByID returns the absolute path to a session directory given its ID.
func SessionDirByID(sessionID string) (string, error) {
	return sessionDir(sessionID)
}

// sessionHasAnyTag reports whether session s has at least one tag in the want list.
func sessionHasAnyTag(s *Session, want []string) bool {
	for _, w := range want {
		for _, t := range s.Tags {
			if strings.EqualFold(t, w) {
				return true
			}
		}
	}
	return false
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func writeMetaJSON(dir string, s *Session) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal session: %w", err)
	}
	path := filepath.Join(dir, "meta.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("cannot write meta.json: %w", err)
	}
	return nil
}
