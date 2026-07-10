package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"unicode"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// ErrExportTemplateBuiltin is returned when mutating a builtin preset.
var ErrExportTemplateBuiltin = errors.New("builtin export template cannot be modified")

// CreateExportTemplateInput creates a user-owned export template.
type CreateExportTemplateInput struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Format      string `json:"format"` // csv | tsv | text
	Body        string `json:"body"`   // JSON TabularTemplateSpec, or Go text/template for text
}

// UpdateExportTemplateInput updates a user-owned export template.
type UpdateExportTemplateInput struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Format      string `json:"format"`
	Body        string `json:"body"`
}

// PreviewExportInput renders a saved template or an unsaved draft body.
type PreviewExportInput struct {
	PeriodID    int64  `json:"periodId"`
	TemplateKey string `json:"templateKey"`
	Format      string `json:"format"`
	Body        string `json:"body"`
}

var exportTemplateKeyPattern = regexp.MustCompile(`[^a-z0-9]+`)

// CreateExportTemplate inserts a custom export template (tabular or text).
func (s *Service) CreateExportTemplate(ctx context.Context, input CreateExportTemplateInput) (ExportTemplate, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ExportTemplate{}, fmt.Errorf("create export template: name is required")
	}
	format, err := normalizeExportFormat(input.Format)
	if err != nil {
		return ExportTemplate{}, fmt.Errorf("create export template: %w", err)
	}
	body, err := normalizeExportBody(input.Body, format)
	if err != nil {
		return ExportTemplate{}, fmt.Errorf("create export template: %w", err)
	}
	key := strings.TrimSpace(input.Key)
	if key == "" {
		key = slugifyExportTemplateKey(name)
	} else {
		key = slugifyExportTemplateKey(key)
	}
	if key == "" {
		return ExportTemplate{}, fmt.Errorf("create export template: key is required")
	}
	key, err = s.uniqueExportTemplateKey(ctx, key)
	if err != nil {
		return ExportTemplate{}, err
	}

	row, err := s.q.CreateExportTemplate(ctx, sqlc.CreateExportTemplateParams{
		Key:         key,
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		Format:      format,
		Builtin:     0,
		Body:        body,
	})
	if err != nil {
		return ExportTemplate{}, mapErr("create export template", err)
	}
	return exportTemplateFromRow(row.ID, row.Key, row.Name, row.Description, row.Format, row.Builtin, row.Body), nil
}

// UpdateExportTemplate updates a custom template. Builtins are rejected.
func (s *Service) UpdateExportTemplate(ctx context.Context, input UpdateExportTemplateInput) (ExportTemplate, error) {
	current, err := s.q.GetExportTemplate(ctx, input.ID)
	if err != nil {
		return ExportTemplate{}, mapErr("update export template", err)
	}
	if current.Builtin != 0 {
		return ExportTemplate{}, fmt.Errorf("update export template: %w", ErrExportTemplateBuiltin)
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ExportTemplate{}, fmt.Errorf("update export template: name is required")
	}
	format, err := normalizeExportFormat(input.Format)
	if err != nil {
		return ExportTemplate{}, fmt.Errorf("update export template: %w", err)
	}
	body, err := normalizeExportBody(input.Body, format)
	if err != nil {
		return ExportTemplate{}, fmt.Errorf("update export template: %w", err)
	}

	row, err := s.q.UpdateExportTemplate(ctx, sqlc.UpdateExportTemplateParams{
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		Format:      format,
		Body:        body,
		ID:          input.ID,
	})
	if err != nil {
		return ExportTemplate{}, mapErr("update export template", err)
	}
	return exportTemplateFromRow(row.ID, row.Key, row.Name, row.Description, row.Format, row.Builtin, row.Body), nil
}

// DeleteExportTemplate removes a custom template. Builtins are rejected.
func (s *Service) DeleteExportTemplate(ctx context.Context, id int64) error {
	current, err := s.q.GetExportTemplate(ctx, id)
	if err != nil {
		return mapErr("delete export template", err)
	}
	if current.Builtin != 0 {
		return fmt.Errorf("delete export template: %w", ErrExportTemplateBuiltin)
	}
	n, err := s.q.DeleteExportTemplate(ctx, id)
	if err != nil {
		return mapErr("delete export template", err)
	}
	if n == 0 {
		return fmt.Errorf("delete export template: %w", ErrNotFound)
	}
	return nil
}

// DuplicateExportTemplate copies any template (including builtins) as a custom row.
func (s *Service) DuplicateExportTemplate(ctx context.Context, key string) (ExportTemplate, error) {
	src, err := s.GetExportTemplate(ctx, key)
	if err != nil {
		return ExportTemplate{}, err
	}
	format := src.Format
	body := src.Body
	switch format {
	case "csv", "tsv":
		if strings.TrimSpace(body) == "" {
			if spec, ok := builtinTabularSpec(src.Key); ok {
				encoded, encErr := encodeTabularSpec(spec)
				if encErr != nil {
					return ExportTemplate{}, encErr
				}
				body = encoded
			}
		}
		normalized, normErr := normalizeTabularBody(body, format)
		if normErr != nil {
			return ExportTemplate{}, fmt.Errorf("duplicate export template: %w", normErr)
		}
		body = normalized
	case "text":
		if strings.TrimSpace(body) == "" {
			body = builtinTextSummaryTemplate
		}
		normalized, normErr := normalizeTextBody(body)
		if normErr != nil {
			return ExportTemplate{}, fmt.Errorf("duplicate export template: %w", normErr)
		}
		body = normalized
	}

	baseKey := slugifyExportTemplateKey(src.Key + "_copy")
	if baseKey == "" {
		baseKey = "export_copy"
	}
	newKey, err := s.uniqueExportTemplateKey(ctx, baseKey)
	if err != nil {
		return ExportTemplate{}, err
	}

	row, err := s.q.CreateExportTemplate(ctx, sqlc.CreateExportTemplateParams{
		Key:         newKey,
		Name:        src.Name + " (copy)",
		Description: src.Description,
		Format:      format,
		Builtin:     0,
		Body:        body,
	})
	if err != nil {
		return ExportTemplate{}, mapErr("duplicate export template", err)
	}
	return exportTemplateFromRow(row.ID, row.Key, row.Name, row.Description, row.Format, row.Builtin, row.Body), nil
}

// PreviewExport renders a saved template or draft body against a period.
func (s *Service) PreviewExport(ctx context.Context, input PreviewExportInput) (PeriodExportRender, error) {
	model, err := s.BuildPeriodExport(ctx, input.PeriodID)
	if err != nil {
		return PeriodExportRender{}, err
	}

	body := input.Body
	if strings.TrimSpace(body) != "" {
		format, err := normalizeExportFormat(input.Format)
		if err != nil {
			return PeriodExportRender{}, fmt.Errorf("preview export: %w", err)
		}
		body, err = normalizeExportBody(body, format)
		if err != nil {
			return PeriodExportRender{}, fmt.Errorf("preview export: %w", err)
		}
		tmpl := ExportTemplate{
			Key:    "preview",
			Name:   "Preview",
			Format: format,
			Body:   body,
		}
		return renderExportTemplate(model, tmpl)
	}

	key := strings.TrimSpace(input.TemplateKey)
	if key == "" {
		key = ExportTemplateMatrixCSV
	}
	tmpl, err := s.GetExportTemplate(ctx, key)
	if err != nil {
		return PeriodExportRender{}, err
	}
	return renderExportTemplate(model, tmpl)
}

func normalizeExportFormat(format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "", "csv":
		return "csv", nil
	case "tsv":
		return "tsv", nil
	case "text":
		return "text", nil
	default:
		return "", fmt.Errorf("format %q is not supported (want csv, tsv, or text)", format)
	}
}

func normalizeExportBody(body, format string) (string, error) {
	switch format {
	case "csv", "tsv":
		return normalizeTabularBody(body, format)
	case "text":
		return normalizeTextBody(body)
	default:
		return "", fmt.Errorf("format %q is not supported", format)
	}
}

func normalizeTextBody(body string) (string, error) {
	body = strings.TrimRight(body, "\n")
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("text template body is required")
	}
	if _, err := template.New("export_text").Funcs(exportTemplateFuncs()).Parse(body); err != nil {
		return "", fmt.Errorf("invalid text template: %w", err)
	}
	return body, nil
}

func normalizeTabularBody(body, format string) (string, error) {
	spec, err := parseTabularSpec(body)
	if err != nil {
		return "", err
	}
	if format == "tsv" {
		spec.Delimiter = "\t"
	} else if spec.Delimiter == "" {
		spec.Delimiter = delimiterFromFormat(format)
	} else if format == "csv" && spec.Delimiter == "\t" {
		// Format wins when explicitly csv.
		spec.Delimiter = ","
	}
	return encodeTabularSpec(spec)
}

func slugifyExportTemplateKey(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	key := strings.Trim(b.String(), "_")
	key = exportTemplateKeyPattern.ReplaceAllString(key, "_")
	key = strings.Trim(key, "_")
	if len(key) > 64 {
		key = key[:64]
		key = strings.Trim(key, "_")
	}
	return key
}

func (s *Service) uniqueExportTemplateKey(ctx context.Context, base string) (string, error) {
	candidate := base
	for i := 0; i < 50; i++ {
		n, err := s.q.ExportTemplateKeyExists(ctx, candidate)
		if err != nil {
			return "", mapErr("check export template key", err)
		}
		if n == 0 {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s_%d", base, i+2)
	}
	return "", fmt.Errorf("create export template: unable to allocate unique key for %q", base)
}

func renderExportTemplate(model PeriodExportModel, tmpl ExportTemplate) (PeriodExportRender, error) {
	switch tmpl.Format {
	case "csv", "tsv":
		spec, err := parseTabularSpec(tmpl.Body)
		if err != nil {
			fallback, ok := builtinTabularSpec(tmpl.Key)
			if !ok {
				return PeriodExportRender{}, fmt.Errorf("export template %q: %w", tmpl.Key, err)
			}
			spec = fallback
			if tmpl.Format == "tsv" {
				spec.Delimiter = "\t"
			}
		}
		filename := defaultExportFilename(model)
		if spec.Delimiter == "\t" || tmpl.Format == "tsv" {
			filename = strings.TrimSuffix(filename, ".csv") + ".tsv"
		}
		format := tmpl.Format
		if format == "" {
			format = formatFromDelimiter(spec.Delimiter)
		}
		return PeriodExportRender{
			Filename: filename,
			Content:  renderTabular(model, spec),
			Format:   format,
		}, nil
	case "text":
		content, err := renderTextSummary(model, tmpl.Body, tmpl.Key)
		if err != nil {
			return PeriodExportRender{}, err
		}
		return PeriodExportRender{
			Filename: defaultExportTextFilename(model),
			Content:  content,
			Format:   tmpl.Format,
		}, nil
	default:
		return PeriodExportRender{}, fmt.Errorf("export template format %q is not supported", tmpl.Format)
	}
}
