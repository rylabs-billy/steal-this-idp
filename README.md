# Steal this IDP!
![steal-this-idp.png](images/steal-this-idp.png)

Configure and deploy and manage the Akamai App Platform with code!

## Overview
This repo packages a basic proof of concept for using Pulumi IaC and ESC to programmatically deploy the [Akamai App Platform](https://apl-docs.net)―a portable framework for building robust, multi-team environments on Kubernetes.

The installation bootstraps a GitOps architecture with an opinionated collection of golden path templates for CNCF projects to cover all the fundamental pillars of a production-ready environment. Beyond the provided templates, the platform is extensible to other CNCF projects and your own custom resources. Build your castle the way you want―everything is open source! 

Jump to [getting started](#getting-started) if you're looking to walk through this as a hands-on lab, or head over to [usage](#usage) if you're already set up.

### Architecture
This project contains Infrastructure as Code (IaC) using the [Pulumi Golang SDK](https://github.com/pulumi/pulumi/tree/master/sdk/go) and [Linode provider](https://www.pulumi.com/registry/packages/linode/).

The Pulumi code is written as two independent [stacks](https://www.pulumi.com/docs/iac/concepts/stacks/), one for managing the [cloud infrastructure](cmd/infra) components, and the other for the [Kubernetes environment](cmd/apl/app). The [Pulumi Automation API](https://www.pulumi.com/docs/iac/using-pulumi/automation-api/) provides the orchestration to run them together as [micro stacks](https://www.pulumi.com/docs/iac/using-pulumi/organizing-projects-stacks/#micro-stacks). Initial ideation stems from the Automation API [examples](https://github.com/pulumi/automation-api-examples/tree/main/go) repository.

Cloud infrastructure (Linode) components include:
- Three-node managed Kubernetes cluster (16GB VMs) 
- Cloud Load Balancer (NodeBalancer in Linode/Akamai terminology)
- Custom DNS configuration
- S3 compatible Object Storage backends
- NVMe Block Storage (PVs)

App Platform components include (but not limited to):
- Pre-configured GitOps (Argo, Tekton, Gitea)
- KMS/SOPS ([Age](https://github.com/FiloSottile/age?tab=readme-ov-file#installation))
- Roles:
  - Platform Admin
  - Team Admin
  - Team members
- Teams
  - Admin and Developer portals
  - Self-service catalogs
  - Build pipelines
  - Code repositories
  - RBAC kubeconfig 
  - Application and infrastructure monitoring
  - IAM (Keycloak)

> [!NOTE]
> In future iterations we'll be moving away from the [SOPS](https://github.com/getsops/sops) in favor of [sealed secrets](https://techdocs.akamai.com/app-platform/docs/team-secrets).

We picked [Pulumi ESC](https://www.pulumi.com/docs/esc/) (Environments, Secrets, and Configuration) as the platform secret store due to its native integration with Pulumi IaC, and to simplify secret rotation in the long term. This requires creating a ESC environment to reference in our stacks.

### Motivation
Consisting mostly of CNCF projects, the App Platform eats the burden of complexity, while being portable, extensible, and fully capable of self-hosting. In early 2025, the [Akamai Cloud Marketplace](https://www.linode.com/marketplace/apps/) team began composing ideas to build our own IDP―purely on the Kubernetes. Albeit still in beta at the time, we quickly realized that App Platform provided exactly that solution, in addition to being a perfect use case for eating our own dog food!

The App Platform itself however, was designed for the UI experience―not the everything-as-code approach that we prefer. This of course presented some "gotchas" to overcome along the way, but that's what makes this so fun! The anticipated, and unforeseen twists and turns is what made Pulumi the IaC tooling of choice. Pulumi provides SDKs in several familiar programming languanges―allowing us to be **programmers** about our infrastructure―rather than forcing us to double as config language engineers. Furthermore, this provides a great deal more flexibility, for all those projects like ours that demand more flexibility!

> [!NOTE]
> This code represents a sister project that is still prototyping. All of this is subject to change.

### Use Cases
- Platform Engineering: \
As it comes baked with the **_teams experience_** and other essentials for an IDP, it can be utilized as such. More specifically, it can functionally be used as one large **_golden path template_** for building an IDP; a solid starting point, baseline, or skeleton.

- Cloud-native Development: \
Any development team in need of a production-grade Kubernetes runtime that "just works" to run their cloud native workloads.

## Usage
After completing [prerequisites](#prerequisites) and running the [setup](#setup) script, you can proceed with using the `aplcli` for deploying the App Platform. This tool is optional, but provided to simplify your usage. The setup script builds the binary in the [automation](cmd/automation) directory, and places a symlink in `$HOME/.local/bin`. We recommend including this in your `$PATH`, if not set already.

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
This code demonstrates deploying the self-managed [App Platform](https://github.com/linode/apl-core) on Linode (Akamai) cloud infrastructure. It can just as well be deployed to any other compliant Kubernetes cluster―self-hosted or on other cloud providers―but that is beyond the scope of this demo for now. If you don't already have a Linode account, see this [guide](https://techdocs.akamai.com/cloud-computing/docs/getting-started) to get started. This code also assumes a custom domain is pointed to [Linode's Name Servers](https://techdocs.akamai.com/cloud-computing/docs/getting-started-with-dns-manager).

2. [Pulumi](https://www.pulumi.com/docs/iac/): \
Pulumi Cloud is the backend for our `dev` stacks, and for [secrets management](https://www.pulumi.com/docs/esc/). If you don't already have a Pulumi account, create your free account [here](https://app.pulumi.com/signup) and check out this [guide](https://www.linode.com/docs/guides/deploy-in-code-with-pulumi/) for getting familiar with Pulumi on Linode.

3. [Go](https://go.dev/doc/install) >= v1.24.1: \
Pulumi IaC and Automation API code is written in Golang with the [v3 SDK](https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v3/go/pulumi).


4. [git-credential-oauth](https://github.com/hickford/git-credential-oauth) (optional): \
Gitea access secured by Keycloak authentication, and `ssh` is not currently supported. This means that to clone or push/pull from a repo, you have to enter the username and password each time. To get around this annoyance, after the initial installation completes, navigate to Gitea and create an [API access token](https://docs.gitea.com/development/api-usage) with the required scopes. Install and configure the `pass-git-helper` in your for local `.gitconfig`. Use the API token in place of your user password.


### Setup
Set environment variables and update shell config with your [Linode API Token](https://techdocs.akamai.com/cloud-computing/docs/manage-personal-access-tokens) and [Pulumi Access Token](https://www.pulumi.com/docs/pulumi-cloud/access-management/access-tokens/#creating-personal-access-tokens), and optionally update the default text editor if using VS Code.

```bash
export LINODE_TOKEN=<TOKEN>
export PULUMI_ACCESS_TOKEN=<TOKEN>
export EDITOR="code"

# ex: update shell profile
echo "# APL Demo" >> ~/.bashrc
echo "LINODE_TOKEN=$LINODE_TOKEN" >> ~/.bashrc
echo "PULUMI_ACCESS_TOKEN=$PULUMI_ACCESS_TOKEN" >> ~/.bashrc
```

Then run the `setup.sh` script with an optional name/label for your App Platform instance. The label defaults to `apl-demo` if no argument is provided. This will interactively prompt for a domain, email, and choice of a Linode region to deploy, and then automates _most_ of the remaining configuration.

```bash
./setup.sh [PLATFORM_NAME]
```

The script ends by opening the ESC environment for editing. If you set `EDITOR="code"` in the previous step, this will appear in a new tab in VS code. Otherwise it opens in your terminal with `vim`. With the editor open, paste the following `pulumiConfig` block under the top level `values` key. A complete example looks something like <a href="https://gist.github.com/rylabs-billy/035029f2b5a8688d977a1505a8855456" target="_blank">this</a>.

```yaml
# bthompso/apl-demo/dev (org/project/stack)
...
pulumiConfig:
    linode:token: ${linode.token}
    apl:domain: ${apl.inputs.domain}
    apl:email: ${apl.inputs.email}
    apl:label: ${apl.inputs.label}
    apl:region: ${apl.inputs.region}
    apl:aplSlug: ${apl.slug.apl}
    apl:infraSlug: ${apl.slug.infra}
    apl:agePublicKey: ${apl.age.publicKey}
    apl:agePrivateKey: ${apl.age.privateKey}
    apl:otomiAdminPassword: ${apl.otomi.adminPassword}
    apl:lokiAdminPassword: ${apl.loki.adminPassword}
    apl:teamDevelopPassword: ${apl.team.develop.password}
```

## Happy Platform Engineering!

We love feedback! Please let us know about your experience with the Akamai App Platform, as well as if you run into any issues using the the example code in this repo. The best way to reach me is by joining the [Linode Greenlight](https://www.linode.com/green-light/) program and messaging me directly in the slack space.

Docs: [apl-docs.net](https://apl-docs.net/) \
APL repo: [github.com/linode/apl-core](https://github.com/linode/apl-core)
