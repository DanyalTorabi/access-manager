{{/*
Expand the name of the chart.
*/}}
{{- define "access-manager.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
Truncates at 63 chars to stay within Kubernetes label length limits.
*/}}
{{- define "access-manager.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{/*
Common labels applied to all resources.
*/}}
{{- define "access-manager.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{ include "access-manager.selectorLabels" . }}
app.kubernetes.io/version: {{ .Values.image.tag | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels (stable subset used in matchLabels — do not change after initial deploy).
*/}}
{{- define "access-manager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "access-manager.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Postgres selector labels.
*/}}
{{- define "access-manager.postgresSelectorLabels" -}}
app.kubernetes.io/name: {{ include "access-manager.name" . }}-postgres
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
