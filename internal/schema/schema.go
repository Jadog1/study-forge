// Package schema provides the MCP-like introspection schema for the sfq CLI.
// AI agents can call `sfq schema` to get a machine-readable JSON description
// of all commands, flags, and the .sfq file format syntax.
package schema

// Root is the top-level schema returned by `sfq schema`.
type Root struct {
	Tool          string        `json:"tool"`
	Version       string        `json:"version"`
	AgentGuidance AgentGuidance `json:"agent_guidance"`
	Commands      []Command     `json:"commands"`
	Syntax        Syntax        `json:"syntax"`
}

// AgentGuidance gives AI agents default behavior rules for using sfq safely.
type AgentGuidance struct {
	DefaultMode string   `json:"default_mode"`
	Notes       []string `json:"notes"`
}

// Command describes a single CLI command.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Args        []Arg  `json:"args,omitempty"`
	Flags       []Flag `json:"flags,omitempty"`
}

// Arg describes a positional argument.
type Arg struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// Flag describes a named flag.
type Flag struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Default     any    `json:"default,omitempty"`
	Description string `json:"description"`
}

// Syntax describes the .sfq file format.
type Syntax struct {
	FileExtension  string         `json:"file_extension"`
	BlockDelimiter string         `json:"block_delimiter"`
	Header         HeaderSchema   `json:"header"`
	QuestionBlock  QuestionSchema `json:"question_block"`
	QuestionTypes  []TypeSchema   `json:"question_types"`
}

// HeaderSchema describes the optional file header block.
type HeaderSchema struct {
	Description string       `json:"description"`
	Fields      []FieldEntry `json:"fields"`
}

// QuestionSchema describes the fields inside a question block.
type QuestionSchema struct {
	Description string       `json:"description"`
	Fields      []FieldEntry `json:"fields"`
}

// FieldEntry describes a single key inside a block.
type FieldEntry struct {
	Key      string `json:"key"`
	Required bool   `json:"required"`
	Type     string `json:"type"`
	Example  string `json:"example,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// TypeSchema describes a specific question type.
type TypeSchema struct {
	Type         string `json:"type"`
	Description  string `json:"description"`
	ChoiceSyntax string `json:"choice_syntax,omitempty"`
	Example      string `json:"example"`
}

// Build returns the full schema for sfq.
func Build(version string) Root {
	return Root{
		Tool:    "sfq",
		Version: version,
		AgentGuidance: AgentGuidance{
			DefaultMode: "static",
			Notes: []string{
				"Unless the user explicitly asks to save answers, persist scores, inspect session history, or start a tracked session, prefer `generate` or `export` instead of `track`.",
				"`track` launches a local HTTP server and saves answers, timing, and final results under ~/.sfq/sessions/.",
				"`generate` and `export` produce static HTML only. They do not persist answers or results for later inspection.",
				"For machine edits to .sfq content, prefer `edit` with a JSON plan rather than free-form text rewrites.",
			},
		},
		Commands: []Command{
			{
				Name:        "edit",
				Description: "Apply a machine-readable JSON edit plan to a .sfq file. Intended for AI agent safe editing workflows.",
				Args:        []Arg{{Name: "file", Type: "string", Required: true}},
				Flags: []Flag{
					{Name: "ops", Type: "string", Description: "Path to JSON edit plan file, or '-' to read plan JSON from stdin. Required."},
					{Name: "output", Type: "string", Description: "Write edited quiz to this path instead of overwriting the input file."},
					{Name: "dry-run", Type: "bool", Default: false, Description: "Print the edited .sfq content to stdout without writing files."},
				},
			},
			{
				Name:        "track",
				Description: "Start a tracked interactive quiz session using the local HTTP server. This mode saves answers, timing, and final score to persistent session history. Use only when the user explicitly wants tracking or saved results.",
				Args:        []Arg{{Name: "file", Type: "string", Required: true}},
				Flags: []Flag{
					{Name: "no-open", Type: "bool", Default: false, Description: "Do not open the browser automatically after starting the tracked local quiz server."},
				},
			},
			{
				Name:        "generate",
				Description: "Parse a .sfq file and generate a self-contained static interactive HTML quiz page. This does not save answers, timing, or results.",
				Args:        []Arg{{Name: "file", Type: "string", Required: true}},
				Flags: []Flag{
					{Name: "output", Type: "string", Description: "Override the output HTML file path. Defaults to <input_basename>.html in the same directory."},
					{Name: "open", Type: "bool", Default: true, Description: "Automatically open the generated HTML in the default browser."},
				},
			},
			{
				Name:        "open",
				Description: "Open the last generated HTML file in the default browser.",
			},
			{
				Name:        "export",
				Description: "Generate static HTML from a .sfq file without opening the browser. Alias for 'generate --no-open'. No results are saved.",
				Args:        []Arg{{Name: "file", Type: "string", Required: true}},
				Flags: []Flag{
					{Name: "output", Type: "string", Description: "Override the output HTML file path."},
				},
			},
			{
				Name:        "validate",
				Description: "Validate an .sfq file for syntax errors. Exits with code 0 on success, 1 on error.",
				Args:        []Arg{{Name: "file", Type: "string", Required: true}},
			},
			{
				Name:        "info",
				Description: "Print the current state: last generated file path and timestamp.",
			},
			{
				Name:        "history",
				Description: "List and browse past tracked quiz sessions. Sessions come from quizzes started with `track` or `retake`.",
				Flags: []Flag{
					{Name: "tag", Type: "[]string", Description: "Filter sessions by one or more quiz tags. Repeat the flag to provide multiple tags."},
					{Name: "since", Type: "string", Description: "Only include sessions on or after this date in YYYY-MM-DD format."},
					{Name: "until", Type: "string", Description: "Only include sessions on or before this date in YYYY-MM-DD format."},
					{Name: "limit", Type: "int", Default: 0, Description: "Maximum number of sessions to return. 0 means all matching sessions."},
					{Name: "offset", Type: "int", Default: 0, Description: "Skip this many matching sessions before returning results."},
					{Name: "json", Type: "bool", Default: false, Description: "Output session history as JSON for programmatic or AI-agent use."},
					{Name: "plain", Type: "bool", Default: false, Description: "Output a plain text table instead of launching the interactive history UI."},
				},
			},
			{
				Name:        "results",
				Description: "Show detailed results for a previously tracked quiz session, including recorded answers and score.",
				Args:        []Arg{{Name: "session-id", Type: "string", Required: true}},
				Flags: []Flag{
					{Name: "json", Type: "bool", Default: false, Description: "Output the session metadata and recorded answers as JSON."},
				},
			},
			{
				Name:        "retake",
				Description: "Start a fresh tracked quiz session using the source file from a previous tracked session.",
				Args:        []Arg{{Name: "session-id", Type: "string", Required: true}},
				Flags: []Flag{
					{Name: "no-open", Type: "bool", Default: false, Description: "Do not open the browser automatically after starting the retake session."},
				},
			},
			{
				Name:        "schema",
				Description: "Print this machine-readable JSON schema describing all commands and the .sfq syntax. Designed for AI agent introspection.",
			},
		},
		Syntax: Syntax{
			FileExtension:  ".sfq",
			BlockDelimiter: "---",
			Header: HeaderSchema{
				Description: "An optional header block before the first --- delimiter. Describes the quiz as a whole.",
				Fields: []FieldEntry{
					{Key: "# Title", Required: false, Type: "string", Example: "# My Quiz Title", Notes: "Markdown H1 line becomes the quiz title."},
					{Key: "author", Required: false, Type: "string", Example: "author: Jane Doe"},
					{Key: "description", Required: false, Type: "string", Example: "description: A quiz about OOP."},
				},
			},
			QuestionBlock: QuestionSchema{
				Description: "Each question is placed between --- delimiters. Fields use a key: value syntax.",
				Fields: []FieldEntry{
					{Key: "id", Required: false, Type: "string", Example: "id: q1", Notes: "Auto-generated as q1, q2, ... if omitted."},
					{Key: "type", Required: false, Type: "string", Example: "type: multiple-choice", Notes: "Inferred automatically if omitted. See question_types."},
					{Key: "title", Required: false, Type: "string", Example: `title: "What is polymorphism?"`, Notes: "Short summary shown in the sidebar navigation."},
					{Key: "hint", Required: false, Type: "string", Example: `hint: "Think about interfaces."`, Notes: "Shown to the user on demand via a Hint button."},
					{Key: "tags", Required: false, Type: "[]string", Example: "tags: [oop, fundamentals]", Notes: "Comma-separated list of tags inside square brackets."},
					{Key: "?", Required: true, Type: "string", Example: "? Which best describes polymorphism?", Notes: "The question prompt. Supports inline markdown. Must be present."},
					{Key: "answer", Required: false, Type: "string", Example: `answer: "Paris"`, Notes: "Expected answer text for short-answer questions."},
					{Key: "explanation", Required: false, Type: "string", Example: "explanation: Polymorphism allows...", Notes: "Shown after the user answers. Supports markdown. Can span multiple lines."},
				},
			},
			QuestionTypes: []TypeSchema{
				{
					Type:         "multiple-choice",
					Description:  "Single correct answer. Rendered as radio buttons.",
					ChoiceSyntax: "- [x] Correct option\n- [ ] Wrong option",
					Example:      "? Which planet is closest to the Sun?\n- [x] Mercury\n- [ ] Venus\n- [ ] Earth",
				},
				{
					Type:         "multi-select",
					Description:  "Multiple correct answers. Rendered as checkboxes.",
					ChoiceSyntax: "- [x] Correct option\n- [x] Also correct\n- [ ] Wrong option",
					Example:      "? Which are primary colors?\n- [x] Red\n- [x] Blue\n- [ ] Green\n- [x] Yellow",
				},
				{
					Type:         "true-false",
					Description:  "Single true/false question. Rendered as two radio buttons.",
					ChoiceSyntax: "- [x] True\n- [ ] False  (or whichever is correct)",
					Example:      "? The Earth is flat.\n- [ ] True\n- [x] False",
				},
				{
					Type:         "multi-true-false",
					Description:  "Multiple statements each marked True or False. Rendered as a table of toggle buttons.",
					ChoiceSyntax: "- [T] Statement that is true\n- [F] Statement that is false",
					Example:      "? Classify each statement:\n- [T] Water boils at 100°C at sea level.\n- [F] The Sun revolves around the Earth.\n- [T] Humans have 46 chromosomes.",
				},
				{
					Type:         "short-answer",
					Description:  "Free-text input field. Optional expected answer for feedback.",
					ChoiceSyntax: "answer: \"Expected answer text\"",
					Example:      "? What is the capital of France?\nanswer: \"Paris\"",
				},
				{
					Type:         "ordering",
					Description:  "Items to place in the correct order. Rendered as a drag-and-drop list.",
					ChoiceSyntax: "1. First item\n2. Second item\n3. Third item",
					Example:      "? Order the steps of the water cycle:\n1. Evaporation\n2. Condensation\n3. Precipitation\n4. Collection",
				},
			},
		},
	}
}
