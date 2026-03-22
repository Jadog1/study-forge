// Package parser implements the .sfq file format parser.
// The .sfq format uses --- delimiters to separate question blocks,
// with a YAML-like key syntax and markdown content inside blocks.
package parser

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// QuizFile represents a parsed .sfq file.
type QuizFile struct {
	Title       string
	Author      string
	Description string
	Questions   []Question
}

// TotalCount returns the number of questions in the quiz file.
func (qf *QuizFile) TotalCount() int { return len(qf.Questions) }

// Question represents a single parsed question block.
type Question struct {
	ID          string
	Type        QuestionType
	Title       string
	Prompt      string
	Hint        string
	Tags        []string
	Choices     []Choice
	Answer      string // for short-answer type
	Explanation string
}

// Choice represents a single answer option.
type Choice struct {
	Text       string
	Correct    bool  // for multiple-choice, multi-select
	TFValue    *bool // for multi-true-false: true=T, false=F
	OrderIndex int   // for ordering type
}

// QuestionType enumerates supported question types.
type QuestionType string

const (
	TypeMultipleChoice QuestionType = "multiple-choice"
	TypeMultiSelect    QuestionType = "multi-select"
	TypeTrueFalse      QuestionType = "true-false"
	TypeMultiTrueFalse QuestionType = "multi-true-false"
	TypeShortAnswer    QuestionType = "short-answer"
	TypeOrdering       QuestionType = "ordering"
)

// NormalizeQuestionType rewrites accepted aliases to the canonical internal type names.
func NormalizeQuestionType(qt QuestionType) QuestionType {
	switch strings.ToLower(strings.TrimSpace(string(qt))) {
	case "multiple-true-false":
		return TypeMultiTrueFalse
	default:
		return QuestionType(strings.TrimSpace(string(qt)))
	}
}

// ParseFile reads and parses an .sfq file from disk.
func ParseFile(path string) (*QuizFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return parseLines(lines)
}

// ParseString parses an .sfq document from a string (useful for testing).
func ParseString(content string) (*QuizFile, error) {
	lines := strings.Split(content, "\n")
	return parseLines(lines)
}

// blockHasContent returns true if any line in the block has non-whitespace content.
// This prevents trailing newlines in files from creating phantom empty question blocks.
func blockHasContent(lines []string) bool {
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			return true
		}
	}
	return false
}

func parseLines(lines []string) (*QuizFile, error) {
	qf := &QuizFile{}

	// Split on --- delimiters
	var blocks [][]string
	var currentBlock []string
	inHeader := true

	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if inHeader {
				// Save header block and start a question block
				blocks = append(blocks, currentBlock)
				currentBlock = nil
				inHeader = false
			} else {
				// End of a question block — only save if it has real content
				if blockHasContent(currentBlock) {
					blocks = append(blocks, currentBlock)
				}
				currentBlock = nil
			}
		} else {
			currentBlock = append(currentBlock, line)
		}
	}
	// Trailing content (if file doesn't end with `---`)
	if blockHasContent(currentBlock) {
		if inHeader {
			// Entire file is a header (no questions)
			blocks = append(blocks, currentBlock)
		} else {
			blocks = append(blocks, currentBlock)
		}
	}

	if len(blocks) == 0 {
		return qf, nil
	}

	// First block is always the file header
	parseHeader(qf, blocks[0])

	// Remaining blocks are question blocks
	for i, block := range blocks[1:] {
		q, err := parseQuestionBlock(block, i+1)
		if err != nil {
			return nil, fmt.Errorf("error in question block %d: %w", i+1, err)
		}
		qf.Questions = append(qf.Questions, q)
	}

	return qf, nil
}

// parseHeader reads the optional header block at the top of a .sfq file.
// Supports:
//
//	# Title line
//	author: ...
//	description: ...
func parseHeader(qf *QuizFile, lines []string) {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			qf.Title = strings.TrimPrefix(trimmed, "# ")
		} else if strings.HasPrefix(trimmed, "author:") {
			qf.Author = strings.TrimSpace(strings.TrimPrefix(trimmed, "author:"))
		} else if strings.HasPrefix(trimmed, "description:") {
			qf.Description = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
		}
	}
}

// parseQuestionBlock parses a single question block (content between two --- delimiters).
func parseQuestionBlock(lines []string, idx int) (Question, error) {
	q := Question{
		ID: fmt.Sprintf("q%d", idx),
	}

	// We process the block line-by-line.
	// Key-value lines at the start/end of the block control metadata.
	// Lines starting with `?` are the question prompt.
	// Lines starting with `- ` are choices.
	// Lines starting with `1.`, `2.` etc. are ordered items.
	// `explanation:` starts a potentially multi-line explanation.

	var promptLines []string
	var explanationLines []string
	inExplanation := false

	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, " \t")

		if inExplanation {
			// Collect multi-line explanation
			explanationLines = append(explanationLines, line)
			continue
		}

		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "id:"):
			q.ID = strings.TrimSpace(strings.TrimPrefix(trimmed, "id:"))

		case strings.HasPrefix(trimmed, "type:"):
			raw := strings.TrimSpace(strings.TrimPrefix(trimmed, "type:"))
			q.Type = NormalizeQuestionType(QuestionType(raw))

		case strings.HasPrefix(trimmed, "title:"):
			raw := strings.TrimSpace(strings.TrimPrefix(trimmed, "title:"))
			q.Title = strings.Trim(raw, `"`)

		case strings.HasPrefix(trimmed, "hint:"):
			raw := strings.TrimSpace(strings.TrimPrefix(trimmed, "hint:"))
			q.Hint = strings.Trim(raw, `"`)

		case strings.HasPrefix(trimmed, "tags:"):
			raw := strings.TrimSpace(strings.TrimPrefix(trimmed, "tags:"))
			raw = strings.Trim(raw, "[]")
			for _, tag := range strings.Split(raw, ",") {
				t := strings.TrimSpace(tag)
				if t != "" {
					q.Tags = append(q.Tags, t)
				}
			}

		case strings.HasPrefix(trimmed, "answer:"):
			raw := strings.TrimSpace(strings.TrimPrefix(trimmed, "answer:"))
			q.Answer = strings.Trim(raw, `"`)

		case strings.HasPrefix(trimmed, "explanation:"):
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "explanation:"))
			if rest != "" {
				explanationLines = append(explanationLines, rest)
			}
			inExplanation = true

		case strings.HasPrefix(trimmed, "? "):
			promptLines = append(promptLines, strings.TrimPrefix(trimmed, "? "))

		case strings.HasPrefix(trimmed, "?"):
			// `?` with no space — still a prompt
			promptLines = append(promptLines, strings.TrimPrefix(trimmed, "?"))

		case len(trimmed) > 3 && trimmed[0] == '-' && trimmed[1] == ' ':
			// Choice line: `- [x] Text`, `- [ ] Text`, `- [T] Text`, `- [F] Text`
			choice, err := parseChoiceLine(trimmed)
			if err != nil {
				return q, err
			}
			q.Choices = append(q.Choices, choice)

		case isOrderingLine(trimmed):
			// Numbered ordering item: `1. Item text`
			c := parseOrderingLine(trimmed, len(q.Choices))
			q.Choices = append(q.Choices, c)

		case trimmed == "":
			// blank line inside prompt — preserve as paragraph break
			if len(promptLines) > 0 {
				promptLines = append(promptLines, "")
			}

		default:
			// Continuation of prompt
			if len(promptLines) > 0 {
				promptLines = append(promptLines, trimmed)
			}
		}
	}

	q.Prompt = strings.Join(promptLines, "\n")
	q.Explanation = strings.Join(explanationLines, "\n")

	// Default type to multiple-choice if choices exist
	if q.Type == "" {
		q.Type = inferType(q)
	}
	q.Type = NormalizeQuestionType(q.Type)
	NormalizeQuestion(&q)

	if q.Prompt == "" {
		return q, fmt.Errorf("question %s has no prompt (missing '?' line)", q.ID)
	}
	if err := validateQuestion(q); err != nil {
		return q, err
	}

	return q, nil
}

// NormalizeQuestion fills in derived fields needed by downstream formatters and renderers.
func NormalizeQuestion(q *Question) {
	if q == nil {
		return
	}
	if q.Type != TypeMultiTrueFalse {
		return
	}
	for i := range q.Choices {
		if q.Choices[i].TFValue != nil {
			continue
		}
		v := q.Choices[i].Correct
		q.Choices[i].TFValue = &v
	}
}

func validateQuestion(q Question) error {
	switch q.Type {
	case TypeMultipleChoice, TypeMultiSelect, TypeTrueFalse, TypeMultiTrueFalse, TypeOrdering:
		if len(q.Choices) == 0 {
			return fmt.Errorf("question %s type %s requires choices, but none were found", q.ID, q.Type)
		}
	}
	return nil
}

func parseChoiceLine(line string) (Choice, error) {
	// Formats:
	//   - [x] Text   -> correct
	//   - [ ] Text   -> incorrect
	//   - [T] Text   -> TRUE for multi-true-false
	//   - [F] Text   -> FALSE for multi-true-false
	if len(line) < 5 {
		return Choice{}, fmt.Errorf("malformed choice line: %q", line)
	}
	// line starts with "- ["
	inner := string(line[2]) // character inside brackets at position 2  ("- [X] ...")
	// Actually our line is "- [x] Text" so index:
	// 0='-', 1=' ', 2='[', 3=marker, 4=']', 5=' ', 6...=text
	if len(line) < 4 || line[2] != '[' {
		return Choice{}, fmt.Errorf("malformed choice line: %q", line)
	}
	marker := string(line[3])
	// Text starts after "] "
	text := ""
	if len(line) > 5 {
		text = strings.TrimSpace(line[5:])
	}

	c := Choice{Text: text}
	switch strings.ToLower(marker) {
	case "x":
		c.Correct = true
	case " ":
		c.Correct = false
	case "t":
		t := true
		c.TFValue = &t
	case "f":
		f := false
		c.TFValue = &f
	default:
		_ = inner
		return Choice{}, fmt.Errorf("unknown choice marker %q in line: %q", marker, line)
	}
	return c, nil
}

func isOrderingLine(line string) bool {
	for i, ch := range line {
		if ch >= '1' && ch <= '9' {
			rest := line[i+1:]
			return strings.HasPrefix(rest, ". ")
		}
		if ch < '0' || ch > '9' {
			break
		}
	}
	return false
}

func parseOrderingLine(line string, idx int) Choice {
	// "1. Some text" -> Choice{Text: "Some text", OrderIndex: 1}
	parts := strings.SplitN(line, ". ", 2)
	text := ""
	if len(parts) == 2 {
		text = strings.TrimSpace(parts[1])
	}
	return Choice{Text: text, OrderIndex: idx + 1, Correct: true}
}

func inferType(q Question) QuestionType {
	if len(q.Choices) == 0 {
		return TypeShortAnswer
	}

	// Check for multi-true-false
	hasTF := false
	for _, c := range q.Choices {
		if c.TFValue != nil {
			hasTF = true
			break
		}
	}
	if hasTF {
		return TypeMultiTrueFalse
	}

	// Check if ordering
	allOrdered := true
	for _, c := range q.Choices {
		if c.OrderIndex == 0 {
			allOrdered = false
			break
		}
	}
	if allOrdered && len(q.Choices) > 1 {
		return TypeOrdering
	}

	// Count correct answers
	correctCount := 0
	for _, c := range q.Choices {
		if c.Correct {
			correctCount++
		}
	}

	if len(q.Choices) == 2 {
		texts := []string{strings.ToLower(q.Choices[0].Text), strings.ToLower(q.Choices[1].Text)}
		if (texts[0] == "true" || texts[0] == "false") && (texts[1] == "true" || texts[1] == "false") {
			return TypeTrueFalse
		}
	}

	if correctCount > 1 {
		return TypeMultiSelect
	}

	return TypeMultipleChoice
}
