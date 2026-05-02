package template

import (
	"fmt"
	"strings"
	"text/template"
)

// RenderedCommand is the outcome of rendering one setup command string.
// On success, Rendered holds the expanded command and Err is nil.
// On failure, Err is non-nil and Rendered is empty — callers MUST skip
// the command, never fall back to Source (a literal "{{ ... }}" would
// otherwise leak into the shell).
type RenderedCommand struct {
	Source   string
	Rendered string
	Err      error
}

// RenderCommands parses and executes each command string as a Go text/template
// with the given data. Commands are rendered independently; a failure on one
// command does not short-circuit the rest. Unknown keys are reported as errors
// thanks to the "missingkey=error" option.
func RenderCommands(commands []string, data *TemplateData) []RenderedCommand {
	results := make([]RenderedCommand, 0, len(commands))
	for _, src := range commands {
		results = append(results, renderOne(src, data))
	}
	return results
}

func renderOne(src string, data *TemplateData) RenderedCommand {
	tmpl, err := template.New("setup_command").Option("missingkey=error").Parse(src)
	if err != nil {
		return RenderedCommand{Source: src, Err: fmt.Errorf("parse command %q: %w", src, err)}
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return RenderedCommand{Source: src, Err: fmt.Errorf("execute template %q: %w", src, err)}
	}

	return RenderedCommand{Source: src, Rendered: buf.String()}
}
