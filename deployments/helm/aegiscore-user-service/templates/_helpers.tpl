{{/*
Expand the chart name.
*/}}
{{- define "aegiscore-user-service.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "aegiscore-user-service.fullname" -}}
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
{{- define "aegiscore-user-service.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "aegiscore-user-service.labels" -}}
helm.sh/chart: {{ include "aegiscore-user-service.chart" . }}
app.kubernetes.io/name: {{ include "aegiscore-user-service.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: aegiscore
{{- end -}}

{{/*
Selector labels for runtime pods.
*/}}
{{- define "aegiscore-user-service.selectorLabels" -}}
app.kubernetes.io/name: {{ include "aegiscore-user-service.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: runtime
{{- end -}}

{{/*
ServiceAccount name.
*/}}
{{- define "aegiscore-user-service.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "aegiscore-user-service.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Nacos source environment shared by the runtime Deployment and RBAC seed Job.
*/}}
{{- define "aegiscore-user-service.nacosEnv" -}}
- name: AEGISCORE_SERVICE
  value: {{ .Values.nacos.service | quote }}
- name: AEGISCORE_NACOS_ADDR
  value: {{ printf "%s.%s.svc.%s:%v" .Values.nacos.server.serviceName .Values.nacos.server.namespace .Values.nacos.server.clusterDomain .Values.nacos.server.port | quote }}
- name: AEGISCORE_NACOS_NAMESPACE
  value: {{ .Values.nacos.configNamespace | quote }}
- name: AEGISCORE_NACOS_GROUP
  value: {{ .Values.nacos.group | quote }}
{{- with .Values.nacos.extraEnv }}
{{ toYaml . }}
{{- end }}
{{- end -}}

{{/*
Image reference.
*/}}
{{- define "aegiscore-user-service.image" -}}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) -}}
{{- end -}}
