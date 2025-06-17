cluster:
  name: apl-demo
  provider: linode
  domainSuffix: {{ .domain }}
otomi:
  adminPassword: '{{ .otomiAdmin }}'
  hasExternalDNS: true
dns:
  domainFilters: 
    - {{ .domain }}
  provider:
    linode:
      apiToken: {{ .token }}
apps:
  alertmanager:
    enabled: true
  argocd:
    resources:
      controller:
        requests:
          cpu: '200m'
          memory: '1Gi'
        limits:
          cpu: '4'
          memory: '8Gi'
  cert-manager:
    issuer: letsencrypt
    stage: production
    email: admin@{{ .domain }}
  grafana:
    enabled: true
  ingress-nginx-platform:
    _rawValues:
      controller:
        service:
          annotations:
            external-dns.alpha.kubernetes.io/ttl: '30'
            service.beta.kubernetes.io/linode-loadbalancer-tags: '{{ .nodebalancerTag }}'
            service.beta.kubernetes.io/linode-loadbalancer-nodebalancer-id: '{{ .nodebalancerId }}'
  harbor:
    enabled: true
  knative:
    enabled: true
  loki:
    enabled: true
    adminPassword: {{ .lokiAdmin }}
  otel:
    enabled: true
  prometheus:
    enabled: true
  rabbitmq:
    enabled: true
  tempo:
    enabled: true 
  trivy:
    enabled: true
  velero:
    enabled: true
    restic:
      enabled: true
kms:
  sops:
    provider: age
    age:
      publicKey: '{{ .ageKey }}'
      privateKey: '{{ .agePrivKey }}'
obj:
  provider:
    linode:
      region: {{ .region }}
      accessKeyId: {{ .accessKey }}
      secretAccessKey: {{ .secretKey }}
      buckets:
        {{- range .buckets }}
        {{ . }}: {{ .prefix }}-{{ . }}
        {{- end }}
platformBackups:
  database:
    gitea:
        enabled: true
        pathSuffix: gitea
        retentionPolicy: 7d
        schedule: 0 0 * * *
    harbor:
        enabled: true
        pathSuffix: harbor
        retentionPolicy: 7d
        schedule: 0 1 * * *
    keycloak:
        enabled: true
        pathSuffix: keycloak
        retentionPolicy: 7d
        schedule: 0 2 * * *
  persistentVolumes:
    linodeApiToken: {{ .token }}
  gitea:
    enabled: true
    retentionPolicy: 7d
    schedule: 0 3 * * *
teamConfig:
  develop:
    settings:
      password: {{ .teamDevelop }}
      id: develop
      selfService:
        teamMembers:
          createServices: true
          editSecurityPolicies: false
          useCloudShell: true
          downloadKubeconfig: true
          downloadDockerLogin: false
      managedMonitoring:
        grafana: true
        alertmanager: true
      networkPolicy:
        egressPublic: true
        ingressPrivate: true
      resourceQuota:
        - name: pods
          value: '50'
        - name: services.loadbalancers
          value: '10'
