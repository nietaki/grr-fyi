{{/*
Expand the name of the chart.
*/}}
{{- define "grr-fyi-chart.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "grr-fyi-chart.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used for the chart label.
*/}}
{{- define "grr-fyi-chart.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "grr-fyi-chart.labels" -}}
helm.sh/chart: {{ include "grr-fyi-chart.chart" . }}
{{ include "grr-fyi-chart.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "grr-fyi-chart.selectorLabels" -}}
app.kubernetes.io/name: {{ include "grr-fyi-chart.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "grr-fyi-chart.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "grr-fyi-chart.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the secret to use
*/}}
{{- define "grr-fyi-chart.secretName" -}}
{{- if .Values.existingSecret }}
{{- .Values.existingSecret }}
{{- else }}
{{- include "grr-fyi-chart.fullname" . }}
{{- end }}
{{- end }}

{{/*
Return true if any secret values are configured (so we know whether to create a Secret)
*/}}
{{- define "grr-fyi-chart.hasSecrets" -}}
{{- if or .Values.captcha.secret .Values.replication.replicaURL .Values.replication.accessKeyID .Values.replication.secretAccessKey }}
{{- true }}
{{- end }}
{{- end }}

{{/*
PVC name for the data volume
*/}}
{{- define "grr-fyi-chart.pvcName" -}}
{{- if .Values.persistence.existingClaim }}
{{- .Values.persistence.existingClaim }}
{{- else }}
{{- include "grr-fyi-chart.fullname" . }}-data
{{- end }}
{{- end }}
