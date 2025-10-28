cluster:
  name: apl-demo
  provider: linode
  domainSuffix: {{ .domain }}
otomi:
  adminPassword: {{ .otomiAdmin }}
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
  jaeger:
    enabled: true
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
  kyverno:
    enabled: true
kms:
  sops:
    provider: age
    age:
      publicKey: '{{ .ageKey }}'
      privateKey: '{{ .agePrivKey }}'
obj:
  provider:
    type: linode
    linode:
      region: {{ .region }}
      accessKeyId: {{ .accessKey }}
      secretAccessKey: {{ .secretKey }}
      buckets:
        {{- range .buckets }}
        {{ . }}: {{ $.prefix }}-{{ . }}
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
    builds:
      - name: nodejs-hello-world-v0-0-1
        imageName: nodejs-hello-world
        tag: v0.0.1
        trigger: false
        mode:
          docker:
            repoUrl: https://github.com/linode/apl-nodejs-helloworld
            revision: HEAD
            path: ./Dockerfile
          type: docker
      - name: demo-java-maven-v0-0-1
        imageName: demo-java-maven
        tag: v0.0.1
        trigger: false
        mode:
          buildpacks:
            repoUrl: https://github.com/buildpacks/samples
            revision: HEAD
            path: apps/java-maven
          type: buildpacks
    services:
      - headers:
          response:
            set: []
        id: 78595314-cdaf-4b60-acc2-3b1a7f80fe2b
        ingressClassName: platform
        name: httpbin
        ownHost: true
        port: 80
      - id: a106eb22-8c06-41b6-ab15-97aafb0888b5
        ingressClassName: platform
        name: nginx-deployment
        ownHost: true
        paths: []
        port: 80
    workloads:
      - name: nodejs-helloworld
        url: https://github.com/linode/apl-nodejs-helloworld.git
        path: chart/hello-world
        revision: HEAD
      - name: nginx-deployment
        path: k8s-deployment
        revision: main
        selectedChart: k8s-deployment
        url: https://github.com/linode/apl-charts.git
  admin:
    services: []
    workloads:
      - name: nodejs-helloworld
        url: https://github.com/linode/apl-nodejs-helloworld.git
        path: chart/hello-world
        revision: HEAD
files:
  env/teams/develop/workloadValues/nodejs-helloworld.yaml: |
    values: |
      image:
        repository: otomi/nodejs-helloworld
        tag: v1.2.13
  env/teams/develop/workloadValues/nginx-deployment.yaml: |
    values: |
      fullnameOverride: nginx-deployment
      image:
        repository: nginxinc/nginx-unprivileged
        tag: stable
      containerPorts:
        - containerPort: 8080
          protocol: TCP
          name: http
      resources:
        requests:
          cpu: 200m
          memory: 32Mi
      autoscaling:
        minReplicas: 2
        maxReplicas: 10
  env/teams/admin/workloadValues/nodejs-helloworld.yaml: |
    values: |
      image:
        repository: otomi/nodejs-helloworld
        tag: v1.2.13
users:
  - email: bthompso@akamai.com
    firstName: Billy
    lastName: Thompson
    isPlatformAdmin: true
    isTeamAdmin: false
    teams: []
    initialPassword: {{ randInitPass }}
  - email: bthompson@linode.com
    firstName: Ryan
    lastName: Thompson
    isPlatformAdmin: false
    isTeamAdmin: true
    teams:
      - develop
    initialPassword: {{ randInitPass }}
  - email: kyle@protonmail.com
    firstName: Jenna
    lastName: Riley
    isPlatformAdmin: false
    isTeamAdmin: false
    teams:
      - develop
    initialPassword: {{ randInitPass }}
  - email: youareit@cryptq.net
    firstName: Anya
    lastName: Maeve
    isPlatformAdmin: false
    isTeamAdmin: false
    teams:
      - develop
    initialPassword: {{ randInitPass }}
