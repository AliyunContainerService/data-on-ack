{{- define "policy.api" }}
{{- if .Capabilities.APIVersions.Has "policy/v1beta1" -}}
v1beta1
{{- else -}}
v1 
{{- end }}
{{- end }}
