package {{.PackageName}}

import (
	"fmt"
	"context"
	"net/http"
)

// {{.ServiceName}} {{.Description | formatDescription }}
{{- if .Since }}
// Since : {{.Since}}
{{- end}}
{{- if .Deprecated }}
// Deprecated
{{- end}}
{{- if .Internal }}
// Internal
{{- end}}
type {{.ServiceName}} struct {
	client *Client
	url string
}

{{- range $index, $element := .Actions}}
{{ template "action" $element}}
{{- end}}

{{- define "action"}}
{{- if .Params }}
{{ template "request" .}}
{{- end}}

// {{ .MethodName }} {{.Description | formatDescription }}
{{- if .Since}}
// Since {{.Since}}
{{- end}}
{{- if len .Changelog }}
//
// Changelog:
	{{- range .Changelog }}
// {{.String}}
	{{- end}}
{{- end}}
{{- if .Deprecated }}
//
// Deprecated since {{.DeprecatedSince}}
{{- end}}
func (s *{{.ServiceName}}) {{.MethodName}} (ctx context.Context{{- if .Params}}, opt *{{.RequestTypeName}}{{- end}}) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s", s.url, "{{.Key}}")
	return s.client.invoke(ctx, {{.Post}}, url, {{- if .Params}} opt {{- else}} nil {{- end}})
}
{{- end}}

{{- define "request"}}
{{$post := .Post}}
type {{.RequestTypeName}} struct {
{{- /* see https://github.com/golang/go/issues/18221#issuecomment-394255883 */}}
{{- range .Params }}
	// {{.Description | formatDescription }}
	{{- if .Since | formatSince }}
	// Since {{ .Since | formatSince }}
	{{- end}}
	{{- if .Internal}}
	// Internal
	{{- end }}
	{{- if .DefaultValue }}
	// Default: {{.DefaultValue}}
	{{- end}}
	{{- if .ExampleValue}}
	// Example: {{.ExampleValue}}
	{{- end }}
	{{- if .PossibleValues }}
	// Possible values: {{- range .PossibleValues}} "{{.}}", {{- end}}
	{{- end}}
	{{- if .Deprecated}}
	// Deprecated since {{.DeprecatedSince.String}}
	{{- end }}
	{{.ParamName}} string {{- if $post }} {{tick}}json:"{{.Key}}{{ if not .Required}},omitempty{{ end }}"{{tick}} {{- else}} {{tick}}url:"{{.Key}}{{ if not .Required}},omitempty{{ end }}"{{tick}} {{- end}}
{{ end}}
}
{{- end}}
