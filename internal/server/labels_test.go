package server

import (
	"strings"
	"testing"
	"text/template"

	"github.com/alexbakker/alertmanager-ntfy/internal/alertmanager"
	"github.com/alexbakker/alertmanager-ntfy/internal/config"
)

func TestRenderLabelsTemplate(t *testing.T) {
	tests := []struct {
		name           string
		templateStr    string
		labels         map[string]string
		expectedTags   []string
		expectError    bool
	}{
		{
			name:        "nil template uses default behavior",
			templateStr: "",
			labels:      map[string]string{"severity": "critical", "service": "api"},
			expectedTags: []string{"severity = critical", "service = api"},
			expectError: false,
		},
		{
			name:        "empty template returns no tags",
			templateStr: "{{/* empty template */}}",
			labels:      map[string]string{"severity": "critical"},
			expectedTags: []string{},
			expectError: false,
		},
		{
			name:        "custom format with colons",
			templateStr: "{{range $key, $value := .}}{{$key}}: {{$value}}{{end}}",
			labels:      map[string]string{"severity": "critical", "service": "api"},
			expectedTags: []string{"severity: critical", "service: api"},
			expectError: false,
		},
		{
			name:        "split function test",
			templateStr: "{{- $items := split (index . \"list\") \",\" -}}{{- range $i, $item := $items -}}{{- if $i }},{{ end -}}{{ trim $item }}{{- end -}}",
			labels:      map[string]string{"list": "item1, item2 , item3"},
			expectedTags: []string{"item1", "item2", "item3"},
			expectError: false,
		},
		{
			name:        "conditional template",
			templateStr: "{{range $key, $value := .}}{{if ne $key \"internal\"}}{{$key}}={{$value}}{{end}}{{end}}",
			labels:      map[string]string{"severity": "critical", "internal": "debug", "service": "api"},
			expectedTags: []string{"severity=critical", "service=api"},
			expectError: false,
		},
		{
			name:        "custom functions test",
			templateStr: "{{range $key, $value := .}}{{if contains $key \"env\"}}{{ upper $key }}={{ lower $value }}{{end}}{{end}}",
			labels:      map[string]string{"environment": "PRODUCTION", "severity": "critical"},
			expectedTags: []string{"ENVIRONMENT=production"},
			expectError: false,
		},
		{
			name: "show_labels with uppercase values",
			templateStr: `{{- if index . "show_labels" -}}
  {{- if ne (index . "show_labels") "" -}}
    {{- $show_list := split (index . "show_labels") "," -}}
    {{- range $i, $key := $show_list -}}
      {{- $key := trim $key -}}
      {{- if and (ne $key "show_labels") (index $ $key) -}}
        {{- if $i }}, {{ end -}}{{ $key }}={{ upper (index $ $key) }}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- else -}}
  {{- range $key, $value := . -}}
    {{- if ne $key "show_labels" -}}{{ $key }}={{ $value }} {{ end -}}
  {{- end -}}
{{- end -}}`,
			labels:      map[string]string{"show_labels": "severity,service", "severity": "critical", "service": "api", "internal": "debug"},
			expectedTags: []string{"severity=CRITICAL", "service=API"},
			expectError: false,
		},
		{
			name: "show_labels not set, uses default behavior",
			templateStr: `{{- if index . "show_labels" -}}
  {{- if ne (index . "show_labels") "" -}}
    {{- $show_list := split (index . "show_labels") "," -}}
    {{- range $i, $key := $show_list -}}
      {{- $key := trim $key -}}
      {{- if and (ne $key "show_labels") (index $ $key) -}}
        {{- if $i }}, {{ end -}}{{ $key }}={{ upper (index $ $key) }}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- else -}}
  {{- range $key, $value := . -}}
    {{- if ne $key "show_labels" -}}{{ $key }}={{ $value }} {{ end -}}
  {{- end -}}
{{- end -}}`,
			labels:      map[string]string{"severity": "critical", "service": "api"},
			expectedTags: []string{"severity=critical ", "service=api "},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a server with the test template
			cfg := &config.Config{
				Ntfy: &config.Ntfy{
					Notification: config.Notification{
						Templates: &config.Templates{},
					},
				},
			}

			// Set up the template if provided
			if tt.templateStr != "" {
				tmpl, err := template.New("").Funcs(config.TemplateFuncs()).Parse(tt.templateStr)
				if err != nil {
					t.Fatalf("Failed to parse template: %v", err)
				}
				cfg.Ntfy.Notification.Templates.Labels = (*config.Template)(tmpl)
			}

			server := &Server{cfg: cfg}
			alert := &alertmanager.Alert{Labels: tt.labels}

			tags, err := server.renderLabelsTemplate(alert)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check expected number of tags
			if len(tags) != len(tt.expectedTags) {
				t.Errorf("Expected %d tags, got %d. Tags: %v, Expected: %v", len(tt.expectedTags), len(tags), tags, tt.expectedTags)
				return
			}

			// Check that all expected tags are present (order may vary due to map iteration)
			tagSet := make(map[string]bool)
			for _, tag := range tags {
				tagSet[strings.TrimSpace(tag)] = true
			}

			for _, expectedTag := range tt.expectedTags {
				if !tagSet[expectedTag] {
					t.Errorf("Expected tag %q not found in %v", expectedTag, tags)
				}
			}
		})
	}
}
