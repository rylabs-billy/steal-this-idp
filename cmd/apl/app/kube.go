package app

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type KubePkg struct {
	pulumi.ResourceState

	KubeStack pulumi.StringOutput `pulumi:"kubeStack"`
}

type KubePkgArgs struct {
	HelmChart HelmOptions
	Pkg       string
	Provider  *kubernetes.Provider
}

type HelmOptions struct {
	Chart           string
	CreateNamespace bool
	DisableWebhooks bool
	Lint            bool
	Name            string
	Namespace       string
	Postrender      string
	Repo            string
	ReuseValues     bool
	Timeout         int
	ValuesFile      string
	ValuesOverride  pulumi.Map
	Verify          bool
	Version         string
	WaitForJobs     bool
}

func NewKubePkg(ctx *pulumi.Context, pkg string, args *KubePkgArgs, opts ...pulumi.ResourceOption) (*KubePkg, error) {
	var kubePkgResource KubePkg
	var chartName string

	helmOpts := args.HelmChart
	provider := args.Provider

	err := ctx.RegisterComponentResource("pkg:index:KubePkg", pkg, &kubePkgResource, opts...)
	if err != nil {
		return nil, err
	}

	if helmOpts.Name != "" {
		chartName = helmOpts.Name
	} else {
		chartName = helmOpts.Chart
	}

	_, err = helm.NewRelease(ctx, helmOpts.Chart, &helm.ReleaseArgs{
		Chart:           pulumi.String(helmOpts.Chart),
		CreateNamespace: pulumi.Bool(helmOpts.CreateNamespace),
		DisableWebhooks: pulumi.Bool(helmOpts.DisableWebhooks),
		Lint:            pulumi.Bool(helmOpts.Lint),
		Name:            pulumi.String(chartName),
		Namespace:       pulumi.String(helmOpts.Namespace),
		Postrender:      pulumi.String(helmOpts.Postrender),
		RepositoryOpts: helm.RepositoryOptsArgs{
			Repo: pulumi.String(helmOpts.Repo),
		},
		ReuseValues: pulumi.Bool(helmOpts.ReuseValues),
		Timeout:     pulumi.Int(helmOpts.Timeout),
		ValueYamlFiles: pulumi.AssetOrArchiveArray{
			pulumi.NewStringAsset(helmOpts.ValuesFile),
			// pulumi.NewFileAsset(helmOpts.ValuesFile),
		},
		Values:      helmOpts.ValuesOverride,
		Verify:      pulumi.Bool(helmOpts.Verify),
		Version:     pulumi.String(helmOpts.Version),
		WaitForJobs: pulumi.Bool(helmOpts.WaitForJobs),
	}, pulumi.Provider(provider), pulumi.IgnoreChanges([]string{"checksum"}), pulumi.Parent(&kubePkgResource))
	if err != nil {
		return nil, err
	}

	kubePkgResource.KubeStack = pulumi.String(chartName).ToStringOutput()
	ctx.RegisterResourceOutputs(&kubePkgResource, pulumi.Map{
		"kubeStack": pulumi.String(chartName),
	})

	return &kubePkgResource, nil
}
