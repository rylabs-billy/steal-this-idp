package app

import (
	"fmt"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	utils "github.com/rylabs-billy/steal-this-idp/utils"
)

const (
	aplVersion = "4.12.1"
	slug       = "bthompso/apl-demo-infra/dev"
)

type StackRef struct {
	Stack *pulumi.StackReference
}

type StackRefFuncs interface {
	Out() pulumi.AnyOutput
	Details() *pulumi.StackReferenceOutputDetails
	Str() pulumi.StringOutput
	Int() pulumi.IntOutput
	Init() *pulumi.StackReference
}

func (st *StackRef) Init(ctx *pulumi.Context, slug string) {
	stk, err := pulumi.NewStackReference(ctx, "infraStackRef", &pulumi.StackReferenceArgs{
		Name: pulumi.String(slug),
	})
	if err != nil {
		msg := fmt.Sprintf("\nerror: failed to create new stack reference for %v", err)
		ctx.Log.Error(msg, nil)
	}
	st.Stack = stk
}

func (st *StackRef) Out(v string) pulumi.AnyOutput {
	return st.Stack.GetOutput(pulumi.String(v))
}

func (st *StackRef) Details(v string) *pulumi.StackReferenceOutputDetails {
	out, err := st.Stack.GetOutputDetails(v)
	if err != nil {
		return nil
	}
	return out
}

func (st *StackRef) Id(v string) pulumi.IDOutput {
	return st.Stack.GetIDOutput(pulumi.String(v))
}

func (st *StackRef) Int(v string) pulumi.IntOutput {
	return st.Stack.GetIntOutput(pulumi.String(v))
}

func (st *StackRef) Str(v string) pulumi.StringOutput {
	return st.Stack.GetStringOutput(pulumi.String(v))
}

type AplResourceInfo struct {
	Resources map[string]interface{}
	Token     string
	Apl       map[string]string
}

func (r *AplResourceInfo) Run(ctx *pulumi.Context) error {
	err := run(ctx, r)
	return err
}

func run(ctx *pulumi.Context, r *AplResourceInfo) error {
	// run func
	var (
		st     StackRef
		dn     = r.Apl["domain"]
		slug   = r.Apl["infraSlug"]
		region = r.Apl["region"]
	)

	st.Init(ctx, slug)
	ipv4 := st.Details("ipv4").Value.(string)
	kubecfg := st.Details("kubeconfig").SecretValue.(string)
	label := st.Details("aplDemoLabel").Value.(string)
	nbId := st.Details("loadbalancerId").Value.(string)
	nbTag := st.Details("loadbalancerTag").Value.(string)
	obj := st.Details("obj").SecretValue.(map[string]interface{})
	objBuckets := st.Details("objBuckets").Value.([]interface{})
	subs := st.Details("subdomains").Value.(map[string]interface{})

	objRegion := fmt.Sprintf("%v-1", region)
	override := map[string]any{
		"region":             objRegion,
		"domain":             dn,
		"token":              r.Token,
		"accessKey":          obj["accessKey"],
		"secretKey":          obj["secretKey"],
		"prefix":             obj["objPrefix"],
		"buckets":            objBuckets,
		"nodebalancerId":     nbId,
		"nodebalancerIpv4":   ipv4,
		"nodebalancerTag":    nbTag,
		"ageKey":             r.Apl["ageKey"],
		"agePrivKey":         r.Apl["agePrivKey"],
		"lokiAdmin":          r.Apl["lokiAdmin"],
		"otomiAdmin":         r.Apl["otomiAdmin"],
		"teamDevelop":        r.Apl["teamDevelop"],
		"platformAdminEmail": r.Apl["email"],
		"platformLabel":      r.Apl["label"],
	}

	k, err := utils.DecodeKubeConfig(label, kubecfg, true)
	if err != nil {
		return err
	}

	provider, err := kubernetes.NewProvider(ctx, "aplKubeProvider", &kubernetes.ProviderArgs{
		Kubeconfig: pulumi.String(k),
	})
	if err != nil {
		return err
	}

	ctx.Export("aplKubeProvider", provider)

	// dns: ensure mission critical subdomains are resolving
	at := subs["auth"].(string)
	auth, err := utils.NewWaitForDns(ctx, at, &utils.WaitForDnsArgs{
		Domain:  at,
		Ip:      ipv4,
		Timeout: 10,
	})
	if err != nil {
		return err
	}

	kc := subs["keycloak"].(string)
	kcloak, err := utils.NewWaitForDns(ctx, kc, &utils.WaitForDnsArgs{
		Domain:  kc,
		Ip:      ipv4,
		Timeout: 10,
	})
	if err != nil {
		return err
	}

	ap := subs["api"].(string)
	api, err := utils.NewWaitForDns(ctx, ap, &utils.WaitForDnsArgs{
		Domain:  ap,
		Ip:      ipv4,
		Timeout: 10,
	})
	if err != nil {
		return err
	}

	// helm: deploy apl chart
	values := utils.YamlTemplate(ctx, "./helm/apl-values.tpl", override)
	aplChart := HelmOptions{
		Chart:           "apl",
		DisableWebhooks: false,
		Name:            "apl",
		Lint:            true,
		Repo:            "https://linode.github.io/apl-core",
		ReuseValues:     true,
		Timeout:         1200,
		ValuesFile:      values,
		Version:         aplVersion,
		WaitForJobs:     false,
	}

	_, err = NewKubePkg(ctx, "aplHelmInstall", &KubePkgArgs{
		HelmChart: aplChart,
		Pkg:       fmt.Sprintf("apl-%s", aplVersion),
		Provider:  provider,
	}, pulumi.DependsOn([]pulumi.Resource{auth, kcloak, api}), pulumi.DeletedWith(provider))
	if err != nil {
		return err
	}

	return nil
}
