package k8s_config

import (
	"fmt"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/rylabs-billy/apl-demo/apl/linode_config"
)

type KubeClusterConfig struct {
	pulumi.ResourceState

	ClusterName pulumi.StringOutput `pulumi:"clusterName"`
}

type KubeClusterConfigArgs struct {
	ClusterResource *linode_config.LinodeLkeCluster
	HelmChart       HelmOptions
	LoadBalancer    *linode_config.StaticLoadBalancer
}

// types to unmarshal the minimum needed values from kubeconfig file
type KubeConfig struct {
	Contexts       []NamedContext `yaml:"contexts"`
	CurrentContext string         `yaml:"current-context"`
}

type NamedContext struct {
	Name    string             `yaml:"name"`
	Context NamedContextConfig `yaml:"context"`
}

type NamedContextConfig struct {
	Cluster   string `yaml:"cluster"`
	Namespace string `yaml:"namespace"`
	User      string `yaml:"user"`
}

// types for other k8s helpers
type K8sOptions struct {
	WriteConfig bool
	Prefix      string
}

type HelmOptions struct {
	Chart           string
	CreateNameSpace bool
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

type KubeSecrets struct {
	Kind      string
	Data      map[string]string
	Name      string
	Namespace string
	Provider  *kubernetes.Provider
}

type KubeNamespace struct {
	Name      string
	Namespace string
	Provider  *kubernetes.Provider
}

func NewKubeClusterConfig(ctx *pulumi.Context, clusterName string, clusterArgs *KubeClusterConfigArgs, opts ...pulumi.ResourceOption) (*KubeClusterConfig, error) {
	var k8sResource KubeClusterConfig
	var chartName string
	// var deleteSvc *local.Command
	args := clusterArgs
	helmOpts := args.HelmChart
	// loadbalancer := args.LoadBalancer
	provider := args.ClusterResource.LkeClusterProvider
	// kubeconfig := args.ClusterResource.LkeCluster.Kubeconfig

	err := ctx.RegisterComponentResource("pkg:index:KubeClusterConfig", clusterName, &k8sResource, opts...)
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
		CreateNamespace: pulumi.Bool(helmOpts.CreateNameSpace),
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
	}, pulumi.Provider(provider),
		pulumi.IgnoreChanges([]string{"checksum"}),
		pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "20m", Update: "20m"}),
		pulumi.Parent(&k8sResource))
	if err != nil {
		msg := fmt.Sprintf("error installing helm chart: %v", err)
		ctx.Log.Error(msg, nil)
	}

	k8sResource.ClusterName = pulumi.String(clusterName).ToStringOutput()
	ctx.RegisterResourceOutputs(&k8sResource, pulumi.Map{
		"clusterName": pulumi.ToOutput(clusterName),
	})

	return &k8sResource, nil
}

// think about this some more...
func (k *KubeClusterConfig) Namespace(ctx *pulumi.Context, args KubeNamespace) {
	CreateNamespace(ctx, args)
}
