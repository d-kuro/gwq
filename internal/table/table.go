// Package table provides a convenient interface for creating styled tables using lipgloss/table.
// It simplifies table creation with predefined styles and a fluent builder API, supporting
// various output formats, color schemes, and border styles.
package table

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Builder provides a convenient interface for creating styled tables using lipgloss/table
type Builder struct {
	headers []string
	rows    [][]string
	style   Style
	output  io.Writer
}

// Style defines the visual styling options for tables
type Style struct {
	Border lipgloss.Border
	// HeaderStyle applies styling to header row
	HeaderStyle lipgloss.Style
	// CellStyle applies styling to data cells
	CellStyle lipgloss.Style
	// Width sets the table width (0 for auto-width)
	Width int
	// MarginLeft sets left margin (default: 1)
	MarginLeft int
	// MarginRight sets right margin (default: 1)
	MarginRight int
	// PaddingLeft sets left padding inside cells (default: 1)
	PaddingLeft int
	// PaddingRight sets right padding inside cells (default: 1)
	PaddingRight int
}

// DefaultStyle returns a clean default style for tables
func DefaultStyle() Style {
	return Style{
		Border: lipgloss.NormalBorder(),
		HeaderStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("8")),
		CellStyle:    lipgloss.NewStyle(),
		Width:        0,
		MarginLeft:   1,
		MarginRight:  1,
		PaddingLeft:  1,
		PaddingRight: 1,
	}
}

// MinimalStyle returns a minimal border style for simple tables
func MinimalStyle() Style {
	return Style{
		Border:       lipgloss.RoundedBorder(),
		HeaderStyle:  lipgloss.NewStyle().Bold(true),
		CellStyle:    lipgloss.NewStyle(),
		Width:        0,
		MarginLeft:   1,
		MarginRight:  1,
		PaddingLeft:  1,
		PaddingRight: 1,
	}
}

// NoBorderStyle returns a borderless style for compact output
func NoBorderStyle() Style {
	return Style{
		Border:       lipgloss.Border{},
		HeaderStyle:  lipgloss.NewStyle().Bold(true).Underline(true),
		CellStyle:    lipgloss.NewStyle(),
		Width:        0,
		MarginLeft:   0,
		MarginRight:  0,
		PaddingLeft:  0,
		PaddingRight: 0,
	}
}

// New creates a new table builder with default styling
func New() *Builder {
	return &Builder{
		style:  DefaultStyle(),
		output: os.Stdout,
	}
}

// NewWithStyle creates a new table builder with custom styling
func NewWithStyle(style Style) *Builder {
	return &Builder{
		style:  style,
		output: os.Stdout,
	}
}

// SetOutput sets the output writer for the table
func (b *Builder) SetOutput(w io.Writer) *Builder {
	b.output = w
	return b
}

// Headers sets the table headers
func (b *Builder) Headers(headers ...string) *Builder {
	b.headers = make([]string, len(headers))
	copy(b.headers, headers)
	return b
}

// Row adds a data row to the table
func (b *Builder) Row(columns ...string) *Builder {
	row := make([]string, len(columns))
	copy(row, columns)
	b.rows = append(b.rows, row)
	return b
}

// Rows adds multiple data rows to the table
func (b *Builder) Rows(rows [][]string) *Builder {
	for _, row := range rows {
		b.Row(row...)
	}
	return b
}

// Build creates and returns the formatted table as a string
func (b *Builder) Build() string {
	t := table.New().
		Border(b.style.Border).
		StyleFunc(func(row, col int) lipgloss.Style {
			// Apply padding to all cells for better readability
			return lipgloss.NewStyle().
				PaddingLeft(b.style.PaddingLeft).
				PaddingRight(b.style.PaddingRight)
		})

	// Set width if specified
	if b.style.Width > 0 {
		t.Width(b.style.Width)
	}

	// Add headers if provided
	if len(b.headers) > 0 {
		t.Headers(b.headers...)
	}

	// Add all rows
	for _, row := range b.rows {
		t.Row(row...)
	}

	// Render the table and apply margin styling
	tableOutput := t.Render()

	// Apply margins based on style configuration
	styledTable := lipgloss.NewStyle().
		MarginLeft(b.style.MarginLeft).
		MarginRight(b.style.MarginRight).
		Render(tableOutput)

	return styledTable
}

// Print writes the table to the configured output writer
func (b *Builder) Print() error {
	_, err := fmt.Fprint(b.output, b.Build())
	return err
}

// Println writes the table followed by a newline to the configured output writer
func (b *Builder) Println() error {
	_, err := fmt.Fprintln(b.output, b.Build())
	return err
}

// WriteCSV writes the table data in CSV format to the output writer
func (b *Builder) WriteCSV() error {
	// Write headers if present
	if len(b.headers) > 0 {
		_, err := fmt.Fprintln(b.output, strings.Join(b.headers, ","))
		if err != nil {
			return err
		}
	}

	// Write data rows
	for _, row := range b.rows {
		// Escape CSV fields that contain commas or quotes
		escapedRow := make([]string, len(row))
		for i, field := range row {
			if strings.Contains(field, ",") || strings.Contains(field, "\"") || strings.Contains(field, "\n") {
				escapedRow[i] = "\"" + strings.ReplaceAll(field, "\"", "\"\"") + "\""
			} else {
				escapedRow[i] = field
			}
		}
		_, err := fmt.Fprintln(b.output, strings.Join(escapedRow, ","))
		if err != nil {
			return err
		}
	}

	return nil
}

// SetMargins sets the left and right margins for the table
func (b *Builder) SetMargins(left, right int) *Builder {
	b.style.MarginLeft = left
	b.style.MarginRight = right
	return b
}

// SetPadding sets the left and right padding inside cells
func (b *Builder) SetPadding(left, right int) *Builder {
	b.style.PaddingLeft = left
	b.style.PaddingRight = right
	return b
}

// Clear resets the table data but keeps the styling
func (b *Builder) Clear() *Builder {
	b.headers = nil
	b.rows = nil
	return b
}

// RowCount returns the number of data rows in the table
func (b *Builder) RowCount() int {
	return len(b.rows)
}

// HasHeaders returns true if headers are set
func (b *Builder) HasHeaders() bool {
	return len(b.headers) > 0
}

// Simple is a convenience function to quickly create and print a table
func Simple(headers []string, rows [][]string) error {
	return New().Headers(headers...).Rows(rows).Println()
}

// SimpleWithStyle is a convenience function to quickly create and print a styled table
func SimpleWithStyle(style Style, headers []string, rows [][]string) error {
	return NewWithStyle(style).Headers(headers...).Rows(rows).Println()
}
