{{- define "master-go-cqrs.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "master-go-cqrs.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name (include "master-go-cqrs.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "master-go-cqrs.serviceName" -}}
{{- printf "%s-%s" (include "master-go-cqrs.fullname" .root) .name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "master-go-cqrs.labels" -}}
app.kubernetes.io/part-of: {{ include "master-go-cqrs.name" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | quote }}
{{- end }}

{{- define "master-go-cqrs.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "master-go-cqrs.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- required "serviceAccount.name is required when serviceAccount.create=false" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

