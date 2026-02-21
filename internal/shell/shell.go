// Package shell provides shell wrapper template rendering for gwq shell integration.
package shell

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

// TemplateData contains the data passed to shell wrapper templates.
type TemplateData struct {
	CommandName string // "gwq"
}

// WriteWrapper renders the shell wrapper function template and writes it to w.
func WriteWrapper(w io.Writer, shellName string, data TemplateData) error {
	templateFile := fmt.Sprintf("templates/%s.tmpl", shellName)

	content, err := templateFS.ReadFile(templateFile)
	if err != nil {
		return fmt.Errorf("unsupported shell: %s", shellName)
	}

	tmpl, err := template.New(shellName).Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse template for %s: %w", shellName, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to render template for %s: %w", shellName, err)
	}

	_, err = w.Write(buf.Bytes())
	return err
}
