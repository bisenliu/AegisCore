{{/*
Expand the chart name.
*/}}
{{- define "aegiscore-user-services.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "aegiscore-user-services.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart label value.
*/}}
{{- define "aegiscore-user-services.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "aegiscore-user-services.labels" -}}
helm.sh/chart: {{ include "aegiscore-user-services.chart" . }}
app.kubernetes.io/name: {{ include "aegiscore-user-services.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: aegiscore
{{- end -}}

{{/*
Selector labels for runtime pods.
*/}}
{{- define "aegiscore-user-services.selectorLabels" -}}
app.kubernetes.io/name: {{ include "aegiscore-user-services.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: runtime
{{- end -}}

{{/*
ServiceAccount name.
*/}}
{{- define "aegiscore-user-services.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "aegiscore-user-services.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Runtime ConfigMap name.
*/}}
{{- define "aegiscore-user-services.configMapName" -}}
{{- printf "%s-config" (include "aegiscore-user-services.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Runtime Secret name. The chart references this Secret but does not render it.
*/}}
{{- define "aegiscore-user-services.secretName" -}}
{{- .Values.secret.existingSecret | default (printf "%s-runtime" (include "aegiscore-user-services.fullname" .)) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Image reference.
*/}}
{{- define "aegiscore-user-services.image" -}}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) -}}
{{- end -}}

{{/*
Secret-backed runtime env.
*/}}
{{- define "aegiscore-user-services.secretEnv" -}}
- name: AEGISCORE_AUTH_JWT_SECRET
  valueFrom:
    secretKeyRef:
      name: {{ include "aegiscore-user-services.secretName" . }}
      key: {{ .Values.secret.keys.jwtSecret }}
- name: AEGISCORE_RESOURCES_POSTGRES_USER_DB_USERNAME
  valueFrom:
    secretKeyRef:
      name: {{ include "aegiscore-user-services.secretName" . }}
      key: {{ .Values.secret.keys.postgresUsername }}
- name: AEGISCORE_RESOURCES_POSTGRES_USER_DB_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "aegiscore-user-services.secretName" . }}
      key: {{ .Values.secret.keys.postgresPassword }}
- name: AEGISCORE_RESOURCES_REDIS_CACHE_REDIS_USERNAME
  valueFrom:
    secretKeyRef:
      name: {{ include "aegiscore-user-services.secretName" . }}
      key: {{ .Values.secret.keys.redisUsername }}
      optional: true
- name: AEGISCORE_RESOURCES_REDIS_CACHE_REDIS_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "aegiscore-user-services.secretName" . }}
      key: {{ .Values.secret.keys.redisPassword }}
      optional: true
{{- end -}}
