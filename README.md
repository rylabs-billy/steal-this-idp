# Steal this IDP!
![steal-this-idp.png](images/steal-this-idp.png)

Deploy and manage the Akamai App Platform with code!

## Overview
This repo packages a basic proof of concept for using Pulumi IaC and ESC to programmatically deploy the [Akamai App Platform](https://apl-docs.net)―a portable framework for building robust, multi-team environments on Kubernetes.

The installation bootstraps a GitOps architecture with an opinionated collection of golden path templates for CNCF projects to cover all the fundamental pillars of a production-ready environment. Beyond the provided templates, the platform is extensible to other CNCF projects and your own custom resources. You can build your castle the way you want―everything is open source!

Jump to [getting started](#getting-started) if you're looking to walk through this as a hands-on lab.

### Motivation
Consisting mostly of CNCF projects, the App Platform eats the burden of complexity, while being portable, extensible, and fully capable of self-hosting. Albeit still in beta (GA date TBA), the [Akamai Cloud Marketplace](https://www.linode.com/marketplace/apps/) team began looking at App Platform for internal use. We found it to be great candidate for own team-level IDP, and a great example of eating our own dog food!

The App Platform itself however, was designed for the UI experience―not the everything-as-code approach that we are taking with it. This of course presents some challenges "gotchas" to overcome along the way, and thus is project is still heavily prototyping, but that's what makes this so fun! As we iterate and iron out the wrinkles as we find them, we also want to share the fun where we can. We forked one such iteration of our progress into this shareable demo, for precisely that reason.

### Use Cases

- Platform Engineering: \
As it comes baked with the **_teams experience_** and other essentials for an IDP, it can be utilized as such. More specifically, it can functionally be used as one large **_golden path template_** for building an IDP; a solid starting point, baseline, or skeleton.

- Cloud-native Development: \
Any development team in need of a production-grade Kubernetes runtime that "just works" to run their cloud native workloads.


## Getting Started

### Prerequisites
1. [Linode](https://www.linode.com/) account: \
This PoC demonstrates deploying the Akamai App Platform on Linode (Akamai) cloud infrastructure. It can certainly be deployed on self-hosted Kubernetes clusters (examples to come...) as well as on other cloud providers, but that is beyond the scope of this demo for now. If you don't already have a Linode account, see this [guide](https://techdocs.akamai.com/cloud-computing/docs/getting-started) to get started.

2. [Pulumi](https://www.pulumi.com/docs/iac/): \
Pulumi Cloud is the backend for our `dev` [stack](https://www.pulumi.com/docs/iac/concepts/stacks/), and for [secrets management](https://www.pulumi.com/docs/esc/). If you don't already have a Pulumi account, create your free account [here](https://app.pulumi.com/signup) and check out this [guide](https://www.linode.com/docs/guides/deploy-in-code-with-pulumi/) for getting familiar with Pulumi on Linode.

3. [Go](https://go.dev/doc/install) >= v1.24.1: \
Pulumi IaC and Automation API code is written in Golang with the [v3 SDK](https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v3/go/pulumi).

4. [Age](https://github.com/FiloSottile/age?tab=readme-ov-file#installation): \
Age is required as the KMS provider for using [SOPS](https://github.com/getsops/sops) to encrypt sensitive values in git repositories.

5. [git-credential-oauth](https://github.com/hickford/git-credential-oauth) (optional): \
Gitea access secured by Keycloak authentication, and `ssh` is not currently supported. This means that to clone or push/pull from a repo, you have to enter the username and password each time. To get around this annoyance, after the initial installation completes, navigate to Gitea and create an [API access token](https://docs.gitea.com/development/api-usage) with the required scopes. Install and configure the `pass-git-helper` in your for local `.gitconfig`. Use the API token in place of your user password.


### Configure Secrets
We picked [Pulumi ESC](https://www.pulumi.com/docs/esc/) (Environments, Secrets, and Configuration) as the platform secret store due to its native integration with Pulumi IaC, and to simplify secret rotation if this becomes a long term solution.

Log in to the Pulumi web app, generate an [access token](https://www.pulumi.com/docs/pulumi-cloud/access-management/access-tokens/#creating-personal-access-tokens). Navigate to the **apl** directory where the `Pulumi.dev.yaml` file is located and then log in to your account with the `pulumi` CLI.

```
cd apl/
pulumi login
```

Using the `esc` CLI, create a new environment to expose to the stack. The shorthand name of our stack in this example is `dev`. The project name is `apl-demo`. The full project-qualified name is `bthompso/apl-demo/dev`.

```
esc env init apl-demo/dev
```

Get a Linode [API Token](https://techdocs.akamai.com/linode-api/reference/get-started) and Linode Object Storage [access keys](https://techdocs.akamai.com/cloud-computing/docs/getting-started-with-object-storage#generate-an-access-keyhttps://techdocs.akamai.com/cloud-computing/docs/getting-started-with-object-storage#generate-an-access-key).

```
esc env set apl-demo/dev linode.token <TOKEN> --secret
esc env set apl-demo/dev linode.objAccessKey <ACCESS_KEY>
esc env set apl-demo/dev linode.objSecretKey <SECRET_KEY> --secret
```

Next let's generate random passwords for the `otomi` user, the `develop` team, and the admin password for [Loki](https://grafana.com/oss/loki/), and then add them to our environment.

```
for i in otomi develop loki; do
  pass=$(uuidgen | md5sum | base64)
  echo "$i: $pass"
done
...
otomi: ZWU3NjVlZGIyMzRhZjYzNTA4ZGM2ZDcxZDU1MDQ1ZGUgIC0K
develop: ZjIxNTc5YWIyNmE1MWUxZDQ0MjVhZTllOGZiZWI2YTUgIC0K
loki: Nzk1ZmJlNzMyOGY1ZjA0ZTVlNzE1ODJhNDJlNGQ3MmIgIC0K
```

```
esc env set apl-demo/dev apl.otomi.adminPassword ZWU3NjVlZGIyMzRhZjYzNTA4ZGM2ZDcxZDU1MDQ1ZGUgIC0K --secret
esc env set apl-demo/dev apl.loki.adminPassword ZjIxNTc5YWIyNmE1MWUxZDQ0MjVhZTllOGZiZWI2YTUgIC0K --secret
esc env set apl-demo/dev apl.team.develop.password Nzk1ZmJlNzMyOGY1ZjA0ZTVlNzE1ODJhNDJlNGQ3MmIgIC0K --secret
```

Since App Platform uses GitOps, we'll configure [SOPS](https://github.com/getsops/sops) with the [Age](https://github.com/FiloSottile/age) provider. Some [external providers](https://apl-docs.net/docs/get-started/installation/sops#use-sops-with-an-external-key-management-service-kms) are supported, but with Age we can keep it vendor agnostic.

Use the `age-keygen` command to generate small explicit keys. Add the public and private keys to the environment as well.

```
age-keygen
...
# created: 2025-05-19T16:14:25-07:00
# public key: age1zeg7pezzjaejx6aa0xhg6fdf3j02e73l0pqsu7a768efr2k6wg5srv5sed
AGE-SECRET-KEY-1HV3LZTJX958J4JXU9EHS3Y0CA9DPFKZGRDWMJQRWW8L5CXV7L6FS96DQSE
```
```
esc env set apl-demo/dev apl.age.publicKey age1zeg7pezzjaejx6aa0xhg6fdf3j02e73l0pqsu7a768efr2k6wg5srv5sed
esc env set apl-demo/dev apl.age.privateKey AGE-SECRET-KEY-1HV3LZTJX958J4JXU9EHS3Y0CA9DPFKZGRDWMJQRWW8L5CXV7L6FS96DQSE
```

Open and edit the environment to map these values for our stack. and update the [stack config]() to use that environment. The below example is using the `pulumi` CLI to do that, instead of the `esc` CLI as we did in the other examples.

> [!Tip]
> You can change the editor (defaults to vim) to VS Code with `export EDITOR="code"`

```
pulumi env edit bthompso/apl-demo/dev
```

With the editor open, add the `pulumiConfig` block under the top level `values` key, and map these values we just set to a variable reference that stacks can use.

```
# bthompso/apl-demo/dev (org/project/stack)
...
pulumiConfig:
    linode:token: ${linode.token}
    linode:objSecretKey: ${linode.objSecretKey}
    linode:objAccessKey: ${linode.objAccessKey}
    apl:agePublicKey: ${apl.age.publicKey}
    apl:agePrivateKey: ${apl.age.privateKey}
    apl:otomiAdminPassword: ${apl.otomi.adminPassword}
    apl:lokiAdminPassword: ${apl.loki.adminPassword}
    apl:teamDevelopPassword: ${apl.team.develop.password}
```

The complete file should look something like the following:
```
values:
  linode:
    token:
      fn::secret:
        ciphertext: ZXNjeAAAAAEAAAEA7C2Oh+tR86aDoKf2W3TNUH4c4WhEVInw93f2bFeJhBp6KVmblfsxkoCVfIbAX1yCPZQqBU7h/bNAW+FL0VfQrj5uks2VwgSbRmIefVZj832Hg9kdT5jIkz4kAZ/o66Ny
    objAccessKey: 27V1GS8ESHI9PFGMZAD2
    objSecretKey:
      fn::secret:
        ciphertext: ZXNjeAAAAAEAAAEAlQiO1B6fUxUHTsCuJTDYvRfwEb+grZmBaLfF/FvTZOaTFk/CkSWXvicKuU2eOgx3OypjeqKEOVw5lIvx1cKkVkI8JqjGoi/V
  pulumiConfig:
    linode:token: ${linode.token}
    linode:objSecretKey: ${linode.objSecretKey}
    linode:objAccessKey: ${linode.objAccessKey}
    apl:agePublicKey: ${apl.age.publicKey}
    apl:agePrivateKey: ${apl.age.privateKey}
    apl:otomiAdminPassword: ${apl.otomi.adminPassword}
    apl:lokiAdminPassword: ${apl.loki.adminPassword}
    apl:teamDevelopPassword: ${apl.team.develop.password}
  apl:
    age:
      publicKey: age1zeg7pezzjaejx6aa0xhg6fdf3j02e73l0pqsu7a768efr2k6wg5srv5sed
      privateKey:
        fn::secret:
          ciphertext: ZXNjeAAAAAEAAAEAeD8/WjpLO9DxPcMMrhKbpxlLHIPqAAZS+5H+mrqw+nlJOLA2sY3hkVCzD9ILofw0THz8lHDDIRYfCACD3u5ui96uFic+atVDuCAVrVXIH7vTovRMlhzftcelg+izktzm9eRxPBeZIWMqKg==
    otomi:
      adminPassword:
        fn::secret:
          ciphertext: ZXNjeAAAAAEAAAEA6KxvcE8T++5NW3GLNduNC8bcpeTpd8OSfQXyO5S0bBHIRIDlcdg0h9eIZrhN0u/t+ISCd8NeKt+N8QNYeqxtDMfEudkluS6tkaS+L7hhOr4=
    loki:
      adminPassword:
        fn::secret:
          ciphertext: ZXNjeAAAAAEAAAEA6/94K5M0EFLHTuLdogIysoDc8eWzQKkQnOdgHed5AdECVavhv1u58uJ71KSo4kSqnvB9MHpBfiJEBQAec5RDXaPCV4lM3/dyTSGT8bqzgsk=
    team:
      develop:
        password:
          fn::secret:
            ciphertext: ZXNjeAAAAAEAAAEAvBMrTuSEc8D+JVO7soQo8a0a7j3sI9rNeIYRVuHLkEI4Nhyl00ilrjSTnbyYS5i8X6va7XJ2J/1xeziegtZC9yAsMhOVB4uOMjhdnd1IhxY=
---
# Please edit the environment definition above.
# The object below is the current result of
# evaluating the environment and will not be
# saved. An empty definition aborts the edit.

{
  "apl": {
    "age": {
      "privateKey": "[unknown]",
      "publicKey": "age1zeg7pezzjaejx6aa0xhg6fdf3j02e73l0pqsu7a768efr2k6wg5srv5sed"
    },
    "loki": {
      "adminPassword": "[unknown]"
    },
    "otomi": {
      "adminPassword": "[unknown]"
    },
    "team": {
      "develop": {
        "password": "[unknown]"
      }
    }
  },
  "linode": {
    "objAccessKey": "27V1GS8ESHI9PFGMZAD2",
    "objSecretKey": "[unknown]",
    "token": "[unknown]"
  },
  "pulumiConfig": {
    "apl:agePrivateKey": "[unknown]",
    "apl:agePublicKey": "age1zeg7pezzjaejx6aa0xhg6fdf3j02e73l0pqsu7a768efr2k6wg5srv5sed",
    "apl:lokiAdminPassword": "[unknown]",
    "apl:otomiAdminPassword": "[unknown]",
    "apl:teamDevelopPassword": "[unknown]",
    "linode:objAccessKey": "27V1GS8ESHI9PFGMZAD2",
    "linode:objSecretKey": "[unknown]",
    "linode:token": "[unknown]",
  },
}
```

Exit the editor to save the changes, then update the local stack file to use the environment―this will be the `Pulumi.dev.yaml`file.

```
# apl-demo/apl/Pulumi.dev.yaml
environment:
  - apl-demo/dev
config:
  pulumi:tags:
    pulumi:template: linode-go
```

Using the `pulumi` CLI, pick just one secret and verify the stack can access and decrypt the value as needed. The below example checks that we can access the Loki admin password, which is mapped as `apl:lokiAdminPassword`.

> [!CAUTION]
> The following command will display a secret value in plain text.

```
pulumi config get apl:lokiAdminPassword
```

If the **plain text** password displays correctly, then everything is working as it should. Rotate this secret and move before moving on the next step.

### Deploy the App Platform
Navigate to the `automation` directory from the repository root. This directory contains code for the [Pulumi Automation API](https://www.pulumi.com/docs/iac/using-pulumi/automation-api/), to which we're using to automate manual invocations of the `pulumi up` command. We're doing this for a couple reasons:

1. Although still heavily prototyping, at some point we'll likely want to break up this monolithic `dev` stack into smaller [micro stacks](https://www.pulumi.com/docs/iac/using-pulumi/organizing-projects-stacks/#micro-stacks), and let the Automation API tie them all together. By implementing it now, we can begin to model what that'll look like.

2. This particular project leverages two pulumi providers―Linode and Kubernetes―with the latter being responsible for creating a cloud resource (NodeBalancer) and the former responsible for it after. For simplicity's sake, to pass resource info from one to the other while strictly controlling the order of operations happening on that resource, calls for two different invocations of `pulumi up`. The Automation API makes this all one fluid motion.

The code organized as four execution steps, which provision the cloud infrastructure resources (LKE, NodeBalancer, DNS, OBJ) and install the App Platform. If all goes smoothly, then this happens in one shot. In the event that a timeout or some other error occurs in the process, it will pick up from last successfully completed stage. Based on the Automation API [examples](https://github.com/pulumi/automation-api-examples/tree/main/go), appending the `destroy` argument will invoke a tear down.

> [!NOTE]
> As a reminder, this code represents a sister project that is still prototyping. All of this is subject to change.

```
go run main.go [destroy]
```

## Happy Platform Engineering!

We love feedback! Please let us know about your experience with the Akamai App Platform, as well as if you run into any issues using the the example code in this repo. The best way to reach me is by joining the [Linode Greenlight](https://www.linode.com/green-light/) program and messaging me directly in the slack space.

Docs: [apl-docs.net](https://apl-docs.net/) \
APL repo: [github.com/linode/apl-core](https://github.com/linode/apl-core)
