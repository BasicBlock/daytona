{{- define "daytona-sandbox-controller.name" -}}
daytona-sandbox-controller
{{- end -}}

{{- define "daytona-sandbox-controller.chart" -}}
{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "daytona-sandbox-controller.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion -}}
{{ .Values.image.repository }}:{{ $tag }}
{{- end -}}

{{- define "daytona-sandbox-controller.labels" -}}
app.kubernetes.io/name: {{ include "daytona-sandbox-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ include "daytona-sandbox-controller.chart" . }}
{{- end -}}

{{- define "daytona-sandbox-controller.controllerLabels" -}}
app.kubernetes.io/name: sandbox-controller
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: controller
{{- end -}}

{{- define "daytona-sandbox-controller.controllerSelectorLabels" -}}
app.kubernetes.io/name: sandbox-controller
{{- end -}}

{{- define "daytona-sandbox-controller.apiLabels" -}}
app.kubernetes.io/name: sandbox-api
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: api
{{- end -}}

{{- define "daytona-sandbox-controller.apiSelectorLabels" -}}
app.kubernetes.io/name: sandbox-api
{{- end -}}
