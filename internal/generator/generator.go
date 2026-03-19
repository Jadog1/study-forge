// Package generator produces a self-contained interactive HTML quiz page
// from a parsed QuizFile.
package generator

import (
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jadog1/study-forge/internal/parser"
	"github.com/yuin/goldmark"
)

//go:embed quiz.html.tmpl
var quizTemplate string

// Options controls generation behaviour.
type Options struct {
	OutputPath string // explicit output path; if empty, derived from source path
}

// Generate produces an HTML file from a parsed QuizFile.
// Returns the absolute path of the generated HTML file.
func Generate(qf *parser.QuizFile, sourcePath string, opts Options) (string, error) {
	outPath := opts.OutputPath
	if outPath == "" {
		dir := filepath.Dir(sourcePath)
		base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
		outPath = filepath.Join(dir, base+".html")
	}

	// Convert all markdown fields to HTML
	data, err := buildTemplateData(qf)
	if err != nil {
		return "", fmt.Errorf("building template data: %w", err)
	}

	tmpl, err := template.New("quiz").Funcs(template.FuncMap{
		"html": func(s string) template.HTML { return template.HTML(s) },
		"add":  func(a, b int) int { return a + b },
	}).Parse(quizTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	absPath, err := filepath.Abs(outPath)
	if err != nil {
		return outPath, nil
	}
	return absPath, nil
}

// TemplateData is the data model passed to the HTML template.
type TemplateData struct {
	Title       string
	Author      string
	Description string
	Questions   []QuestionData
	TotalCount  int
}

// QuestionData is the per-question data model for the template.
type QuestionData struct {
	ID          string
	Index       int // 1-based
	Type        string
	TypeLabel   string
	Title       string
	PromptHTML  template.HTML
	Hint        string
	HasHint     bool
	Tags        []string
	Choices     []ChoiceData
	AnswerText  string // for short-answer
	ExplainHTML template.HTML
	HasExplain  bool
}

// ChoiceData is the per-choice model for the template.
type ChoiceData struct {
	Text      string
	Correct   bool
	TFCorrect string // for multi-true-false: "true", "false", or "" if not applicable
	Order     int
}

func buildTemplateData(qf *parser.QuizFile) (*TemplateData, error) {
	md := goldmark.New()

	renderMD := func(src string) (template.HTML, error) {
		if strings.TrimSpace(src) == "" {
			return "", nil
		}
		var buf strings.Builder
		if err := md.Convert([]byte(src), &buf); err != nil {
			return "", err
		}
		return template.HTML(buf.String()), nil
	}

	title := qf.Title
	if title == "" {
		title = "Quiz"
	}

	data := &TemplateData{
		Title:       title,
		Author:      qf.Author,
		Description: qf.Description,
		TotalCount:  len(qf.Questions),
	}

	typeLabels := map[parser.QuestionType]string{
		parser.TypeMultipleChoice: "Multiple Choice",
		parser.TypeMultiSelect:    "Multi-Select",
		parser.TypeTrueFalse:      "True / False",
		parser.TypeMultiTrueFalse: "True / False (Multi)",
		parser.TypeShortAnswer:    "Short Answer",
		parser.TypeOrdering:       "Ordering",
	}

	for i, q := range qf.Questions {
		promptHTML, err := renderMD(q.Prompt)
		if err != nil {
			return nil, fmt.Errorf("question %s prompt markdown: %w", q.ID, err)
		}
		explainHTML, err := renderMD(q.Explanation)
		if err != nil {
			return nil, fmt.Errorf("question %s explanation markdown: %w", q.ID, err)
		}

		qd := QuestionData{
			ID:          q.ID,
			Index:       i + 1,
			Type:        string(q.Type),
			TypeLabel:   typeLabels[q.Type],
			Title:       q.Title,
			PromptHTML:  promptHTML,
			Hint:        q.Hint,
			HasHint:     q.Hint != "",
			Tags:        q.Tags,
			AnswerText:  q.Answer,
			ExplainHTML: explainHTML,
			HasExplain:  strings.TrimSpace(q.Explanation) != "",
		}
		if qd.Title == "" {
			qd.Title = fmt.Sprintf("Question %d", i+1)
		}
		if qd.TypeLabel == "" {
			qd.TypeLabel = string(q.Type)
		}
		for _, c := range q.Choices {
			cd := ChoiceData{
				Text:    c.Text,
				Correct: c.Correct,
				Order:   c.OrderIndex,
			}
			if c.TFValue != nil {
				if *c.TFValue {
					cd.TFCorrect = "true"
				} else {
					cd.TFCorrect = "false"
				}
			}
			qd.Choices = append(qd.Choices, cd)
		}

		data.Questions = append(data.Questions, qd)
	}
	return data, nil
}
