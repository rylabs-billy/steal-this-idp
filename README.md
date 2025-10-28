# Steal this IDP!
![steal-this-idp.png](images/steal-this-idp.png)

Configure and deploy and manage the Akamai App Platform with code!

## Overview
This repo packages a basic proof of concept for using Pulumi IaC and ESC to programmatically deploy the [Akamai App Platform](https://apl-docs.net)―a portable framework for building robust, multi-team environments on Kubernetes.

The installation bootstraps a GitOps architecture with an opinionated collection of golden path templates for CNCF projects to cover all the fundamental pillars of a production-ready environment. Beyond the provided templates, the platform is extensible to other CNCF projects and your own custom resources. Build your castle the way you want―everything is open source! 

Jump to [getting started](#getting-started) if you're looking to walk through this as a hands-on lab, or head over to [usage](#usage) if you're already set up.

### Motivation
Consisting mostly of CNCF projects, the App Platform eats the burden of complexity, while being portable, extensible, and fully capable of self-hosting. Albeit still in beta at the time, The [Akamai Cloud Marketplace](https://www.linode.com/marketplace/apps/) team began looking at App Platform for internal use. We found it to be great candidate for own team-level IDP, and a great example of eating our own dog food!

The App Platform itself however, was designed for the UI experience―not the everything-as-code approach that we (Marketplace team) prefers. This of course presented some "gotchas" to overcome along the way, and thus is project is still prototyping, but that's what makes this so fun! As we iterate and iron out the wrinkles, we also want to share the fun where we can. This repos will receive semi-regular updates as we sanitize and port over iterations from the internal project.

### Use Cases

- Platform Engineering: \
As it comes baked with the **_teams experience_** and other essentials for an IDP, it can be utilized as such. More specifically, it can functionally be used as one large **_golden path template_** for building an IDP; a solid starting point, baseline, or skeleton.

- Cloud-native Development: \
Any development team in need of a production-grade Kubernetes runtime that "just works" to run their cloud native workloads.

## Usage
After meeting all [prerequisites](#prerequisites), and running the `setup.sh` script for [environment configuration](#configure-environment), you can proceed with using the `aplcli` binary for deploying or destroying the App Platform. Of course you can still do it like `go run main.go...` (I won't judge you) but this CLI tool is provided to simplify your life. It lives in the [automation]() directory. The initial setup script builds it and places a symlink in `$HOME/.local/bin`, and will prompt you to update your `$PATH` accordingly.

Simply Provide a required argument of `create` or `destroy`. For dev and testing purposes, you can also provide an option flag to target a specific stack, but in general it's best to run **_without_** options and leave the program to handle the up/down ordering.

```bash
 usage:        aplcli  [ARG]  [OPTION]  

 description:  run without options to target all stacks, or provide a specific stack name    

 arguments:                  
               create      deploy a stack by name, or run without options to deploy them all  
               destroy     provide a stack name to destroy, or leave blank to destroy everything  

 options:                    
               -a,  --apl    
               -i,  --infra
```

**Examples:**
```bash
# provision all
aplcli create

# provision only a specific stack
aplcli create --infra

# destroy only a specific stack
aplcli destroy --apl

# destroy all
aplcli destroy
```

## Getting Started

### Prerequisites
1. [Linode](https://www.linode.com/) account: \
This code demonstrates deploying the self-managed [App Platform](https://github.com/linode/apl-core) on Linode (Akamai) cloud infrastructure. It can just as well be deployed to any other compliant Kubernetes cluster―self-hosted or on other cloud providers―but that is beyond the scope of this demo for now. If you don't already have a Linode account, see this [guide](https://techdocs.akamai.com/cloud-computing/docs/getting-started) to get started.

2. [Pulumi](https://www.pulumi.com/docs/iac/): \
Pulumi Cloud is the backend for our `dev` [stacks](https://www.pulumi.com/docs/iac/concepts/stacks/), and for [secrets management](https://www.pulumi.com/docs/esc/). If you don't already have a Pulumi account, create your free account [here](https://app.pulumi.com/signup) and check out this [guide](https://www.linode.com/docs/guides/deploy-in-code-with-pulumi/) for getting familiar with Pulumi on Linode.

3. [Go](https://go.dev/doc/install) >= v1.24.1: \
Pulumi IaC and Automation API code is written in Golang with the [v3 SDK](https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v3/go/pulumi).

4. [git-credential-oauth](https://github.com/hickford/git-credential-oauth) (optional): \
Gitea access secured by Keycloak authentication, and `ssh` is not currently supported. This means that to clone or push/pull from a repo, you have to enter the username and password each time. To get around this annoyance, after the initial installation completes, navigate to Gitea and create an [API access token](https://docs.gitea.com/development/api-usage) with the required scopes. Install and configure the `pass-git-helper` in your for local `.gitconfig`. Use the API token in place of your user password.


### Configure Environment
We picked [Pulumi ESC](https://www.pulumi.com/docs/esc/) (Environments, Secrets, and Configuration) as the platform secret store due to its native integration with Pulumi IaC, and to simplify secret rotation if this becomes a long term solution. This requires creating a ESC environment to reference in our Pulumi stack.

We'll also want to create new keys for Linode Object Storage, as well as [Age](https://github.com/FiloSottile/age?tab=readme-ov-file#installation)―the KMS provider for using [SOPS](https://github.com/getsops/sops) to encrypt sensitive values in git repositories.

> [!NOTE]
> In future iterations we'll be moving away from the SOPS in favor of [sealed secrets](https://techdocs.akamai.com/app-platform/docs/team-secrets).

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

## Explanation of Code

This project contains Infrastructure as Code (IaC) using the Linode provider from Pulumi's Golang SDK, and is written to leverage the [Pulumi Automation API](https://www.pulumi.com/docs/iac/using-pulumi/automation-api/) and the underlying mechanism.

The Pulumi code is divided into two separate stacks, one for managing the cloud infrastructure components, and the others for managing the Kubernetes configuration of the App Platform. Each of these stacks can be ran and versioned independently, making them function as [micro stacks](https://www.pulumi.com/docs/iac/using-pulumi/organizing-projects-stacks/#micro-stacks) in Pulumi terms. The initial ideation stems from the Automation API [examples](https://github.com/pulumi/automation-api-examples/tree/main/go) repository.


The cloud infrastructure (Linode) components include:
- Three-node managed Kubernetes cluster (16GB VMs) 
- Cloud Load Balancer (NodeBalancer in Linode terms)
- Custom domain and DNS configuration
- S3 compatible Object Storage
- NVMe Block Storage

> [!NOTE]
> This code represents a sister project that is still prototyping. All of this is subject to change.


## Happy Platform Engineering!

We love feedback! Please let us know about your experience with the Akamai App Platform, as well as if you run into any issues using the the example code in this repo. The best way to reach me is by joining the [Linode Greenlight](https://www.linode.com/green-light/) program and messaging me directly in the slack space.

Docs: [apl-docs.net](https://apl-docs.net/) \
APL repo: [github.com/linode/apl-core](https://github.com/linode/apl-core)
