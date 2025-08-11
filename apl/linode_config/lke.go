package linode_config

import (
	"fmt"

	utils "github.com/rylabs-billy/apl-demo/apl/internal"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"

	"github.com/pulumi/pulumi-linode/sdk/v4/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type LinodeLkeCluster struct {
	pulumi.ResourceState

	LkeCluster         *linode.LkeCluster   `pulumi:"lkeCluster"`
	LkeClusterLabel    string               `pulumi:"lkeLabel"`
	LkeClusterProvider *kubernetes.Provider `pulumi:"lkeClusterProvider"`
	LkeLoadBalancer    *StaticLoadBalancer  `pulumi:"lkeLoadBalancer"`
}

type LinodeLkeClusterArgs struct {
	K8sVersion         string
	NodePools          []*LinodeLkeNodePool
	Region             string
	StaticLoadBalancer bool
}

type LinodeLkeNodePool struct {
	Autoscale  bool
	Count      int
	Max        int
	NodeLabels map[string]string
	Tags       []string
	Type       string
}

func (c *LinodeLkeClusterArgs) LkeDefaults() {
	if c.K8sVersion == "" {
		c.K8sVersion = "1.33"
	}
	if c.Region == "" {
		c.Region = "us-ord"
	}
	if len(c.NodePools) == 0 {
		var np LinodeLkeNodePool
		np.NodePoolDefaults()
		c.NodePools = append(c.NodePools, &np)
	}
}

func (n *LinodeLkeNodePool) NodePoolDefaults() {
	if n.Autoscale {
		if n.Max == 0 {
			n.Max = 15
		}
	}
	if n.Count == 0 {
		n.Count = 3
	}

}

func NewLinodeLkeCluster(ctx *pulumi.Context, lkeClusterLabel string, lkeClusterArgs *LinodeLkeClusterArgs, opts ...pulumi.ResourceOption) (*LinodeLkeCluster, error) {
	var lkeResource LinodeLkeCluster
	args := lkeClusterArgs
	args.LkeDefaults()

	err := ctx.RegisterComponentResource("pkg:index:LinodeLkeCluster", lkeClusterLabel, &lkeResource, opts...)
	if err != nil {
		return nil, err
	}

	// configure node pools
	nodePools := linode.LkeClusterPoolArray{}
	for _, item := range args.NodePools {
		var (
			autoscale linode.LkeClusterPoolAutoscalerArgs
			nodeTags  pulumi.StringArray
		)
		item.NodePoolDefaults()

		if item.Autoscale {
			autoscale = linode.LkeClusterPoolAutoscalerArgs{
				Max: pulumi.Int(item.Max),
				Min: pulumi.Int(item.Count),
			}
		}
		nodeLabels := pulumi.StringMap{}
		for k, v := range item.NodeLabels {
			nodeLabels[k] = pulumi.String(v)
		}
		for _, tag := range item.Tags {
			nodeTags = append(nodeTags, pulumi.String(tag))
		}

		np := &linode.LkeClusterPoolArgs{
			Type:       pulumi.String(item.Type),
			Autoscaler: autoscale,
			Count:      pulumi.Int(item.Count),
			Labels:     nodeLabels,
			Tags:       nodeTags,
		}

		nodePools = append(nodePools, np)
	}

	// deploy lke cluster
	k8sCluster, err := linode.NewLkeCluster(ctx, lkeClusterLabel, &linode.LkeClusterArgs{
		K8sVersion: pulumi.String(args.K8sVersion),
		Label:      pulumi.String(lkeClusterLabel),
		Pools:      nodePools,
		ControlPlane: &linode.LkeClusterControlPlaneArgs{
			HighAvailability: pulumi.Bool(true),
		},
		Region: pulumi.String(args.Region),
	}, pulumi.Parent(&lkeResource), pulumi.IgnoreChanges([]string{"tier"}))
	if err != nil {
		return nil, err
	}

	// create provider
	kubecfg := utils.DecodeKubeconfig(k8sCluster, true)
	provider, err := kubernetes.NewProvider(ctx, "lkeProvider", &kubernetes.ProviderArgs{
		Kubeconfig: kubecfg,
	}, pulumi.Parent(&lkeResource))
	if err != nil {
		msg := fmt.Sprintf("error instantiating kubernetes provider: %v", err)
		ctx.Log.Error(msg, nil)
	}

	lkeResource.LkeCluster = k8sCluster
	lkeResource.LkeClusterLabel = lkeClusterLabel
	lkeResource.LkeClusterProvider = provider
	ctx.RegisterResourceOutputs(&lkeResource, pulumi.Map{
		"lkeCluster":      pulumi.ToOutput(k8sCluster),
		"lkeClusterLabel": pulumi.ToOutput(lkeClusterLabel),
	})

	return &lkeResource, nil
}

func (c *LinodeLkeCluster) StaticLoadBalancer(lb *StaticLoadBalancer) {
	c.LkeLoadBalancer = lb
}
