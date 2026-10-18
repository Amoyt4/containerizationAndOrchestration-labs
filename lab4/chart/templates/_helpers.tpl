{{- define "shop.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "shop.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "shop.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "shop.labels" -}}
app.kubernetes.io/name: {{ include "shop.name" . }}
app.kubernetes.io/part-of: shop
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end -}}

{{- define "shop.selectorLabels" -}}
app.kubernetes.io/name: {{ include "shop.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}