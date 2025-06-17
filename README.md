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

4. [git-credential-oauth](https://github.com/hickford/git-credential-oauth) (optional): \
Gitea access secured by Keycloak authentication, and `ssh` is not currently supported. This means that to clone or push/pull from a repo, you have to enter the username and password each time. To get around this annoyance, after the initial installation completes, navigate to Gitea and create an [API access token](https://docs.gitea.com/development/api-usage) with the required scopes. Install and configure the `pass-git-helper` in your for local `.gitconfig`. Use the API token in place of your user password.


### Configure Environment
We picked [Pulumi ESC](https://www.pulumi.com/docs/esc/) (Environments, Secrets, and Configuration) as the platform secret store due to its native integration with Pulumi IaC, and to simplify secret rotation if this becomes a long term solution. This requires creating a ESC environment to reference in our Pulumi stack.

We'll also want to create new keys for Linode Object Storage, as well as [Age](https://github.com/FiloSottile/age?tab=readme-ov-file#installation)―the KMS provider for using [SOPS](https://github.com/getsops/sops) to encrypt sensitive values in git repositories.

The `setup.sh` script is provided to make your life easier by automating _most_ of this initial setup. 😉 When complete, it will have also generated admin passwords for:
- Develop (team)
- Otomi (Keycloak admin)
- Loki


> [!NOTE]
> Before running the setup script, ensure that you have both a Pulumi Cloud [access token](https://www.pulumi.com/docs/pulumi-cloud/access-management/access-tokens/#creating-personal-access-tokens), and a [Linode API token](https://techdocs.akamai.com/cloud-computing/docs/manage-personal-access-tokens).
>
> For **demo purposes**, give the Linode API key full read/write on all scopes, with an expiration of one month. In production, you'd ideally want to narrow scopes as much as possible. See [example](https://techdocs.akamai.com/cloud-computing/docs/manage-personal-access-tokens#create-an-api-token).

The script will interactively prompt you to paste your Linode and Pulumi tokens. Alternatively you can set these values as `PULUMI_ACCESS_TOKEN` and `LINODE_TOKEN` environment variables prior to execution. 

```bash
# optional: change editor to VS code (defaults to vim)
export EDITOR="code"

# run setup
./setup.sh
```

The script ends by opening the ESC environment for editing. If you set `EDITOR="code"` in the previous step, this will appear in a new tab in VS code. Otherwise it simply opens in your terminal with `vim`.

This manual step, is to map the environment values to references for our stack to consume. With the editor open, add the following `pulumiConfig` block under the top level `values` key.

```yaml
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

The complete file should look something like this <a href="https://gist.github.com/rylabs-billy/035029f2b5a8688d977a1505a8855456" target="_blank">example</a>.


### Deploy the App Platform
Navigate to the `automation` directory from the repository root. This directory contains code for the [Pulumi Automation API](https://www.pulumi.com/docs/iac/using-pulumi/automation-api/), to which we're using to automate manual invocations of the `pulumi up` command. We're doing this for a couple reasons:

1. Although still heavily prototyping, at some point we'll likely want to break up this monolithic `dev` stack into smaller [micro stacks](https://www.pulumi.com/docs/iac/using-pulumi/organizing-projects-stacks/#micro-stacks), and let the Automation API tie them all together. By implementing it now, we can begin to model what that'll look like.

2. This particular project leverages two Pulumi providers―Linode and Kubernetes―with the latter being responsible for creating a cloud resource (NodeBalancer) and the former responsible for it after. For simplicity's sake, to pass resource info from one to the other while strictly controlling the order of operations happening on that resource, calls for two different invocations of `pulumi up`. The Automation API makes this all one fluid motion.

The code organized as four execution steps, which provision the cloud infrastructure resources (LKE, NodeBalancer, DNS, OBJ) and install the App Platform. If all goes smoothly, then this happens in one shot. In the event that a timeout or some other error occurs in the process, it will pick up from last successfully completed stage. Based on the Automation API [examples](https://github.com/pulumi/automation-api-examples/tree/main/go), appending the `destroy` argument will invoke a tear down.

> [!NOTE]
> As a reminder, this code represents a sister project that is still prototyping. All of this is subject to change.

```go
go run main.go [destroy]
```

## Happy Platform Engineering!

We love feedback! Please let us know about your experience with the Akamai App Platform, as well as if you run into any issues using the the example code in this repo. The best way to reach me is by joining the [Linode Greenlight](https://www.linode.com/green-light/) program and messaging me directly in the slack space.

Docs: [apl-docs.net](https://apl-docs.net/) \
APL repo: [github.com/linode/apl-core](https://github.com/linode/apl-core)
