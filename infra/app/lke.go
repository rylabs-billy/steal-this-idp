package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	utils "github.com/rylabs-billy/steal-this-idp/tree/refactor/micro-stacks/infra/internal"

	"gopkg.in/yaml.v2"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-linode/sdk/v4/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type LinodeLkeCluster struct {
	pulumi.ResourceState

	Cluster      *linode.LkeCluster     `pulumi:"lkeCluster"`
	Kubeconfig   map[string]interface{} `pulumi:"lkeKubeconfig"`
	Label        string                 `pulumi:"lkeLabel"`
	Provider     *kubernetes.Provider   `pulumi:"lkeClusterProvider"`
	LoadBalancer *StaticLoadBalancer    `pulumi:"lkeLoadBalancer"`
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

func (c *LinodeLkeCluster) DecodeKubeconfig() *LinodeLkeCluster {
	// var data map[string]interface{}
	j, _ := c.Cluster.Kubeconfig.MarshalJSON()
	fmt.Printf("\n\ntesting print of: %v\n\n", string(j)) // remove
	dec, _ := base64.StdEncoding.DecodeString(string(j))
	yaml.Unmarshal([]byte(dec), c.Kubeconfig)
	// c.Kubeconfig = data
	return c
}

func (c *LinodeLkeCluster) WriteKubeconfig() error {
	homeDir := os.Getenv("HOME")
	fileName := fmt.Sprintf("%s-kubeconfig.yaml", c.Label)
	file := filepath.Join(homeDir, ".kube", fileName)

	if !utils.AssertResource(c.Kubeconfig) {
		_ = c.DecodeKubeconfig()
	}

	j, err := json.Marshal(c.Kubeconfig)
	if err != nil {
		return err
	}

	err = utils.WriteFile(file, j)
	if err != nil {
		return err
	}

	return nil
}

func (c *LinodeLkeCluster) StaticLoadBalancer(lb *StaticLoadBalancer) {
	c.LoadBalancer = lb
}

// func decodeKubeconfig(cluster *linode.LkeCluster, writeConfig bool) pulumi.StringOutput {
// 	decodedKubeconfig := cluster.Kubeconfig.ApplyT(func(k string) (string, error) {
// 		decoded, err := base64.StdEncoding.DecodeString(k)
// 		if err != nil {
// 			return "", err
// 		}
// 		return string(decoded), nil
// 	}).(pulumi.StringOutput)

// 	if writeConfig {
// 		_ = cluster.Label.ApplyT(func(k string) error {
// 			WriteKubeConfig(decodedKubeconfig, k)
// 			return nil
// 		})
// 	}
// 	return decodedKubeconfig
// }

func NewLinodeLkeCluster(ctx *pulumi.Context, label string, lkeClusterArgs *LinodeLkeClusterArgs, opts ...pulumi.ResourceOption) (*LinodeLkeCluster, error) {
	var lkeResource LinodeLkeCluster
	args := lkeClusterArgs
	args.LkeDefaults()

	err := ctx.RegisterComponentResource("pkg:index:LinodeLkeCluster", label, &lkeResource, opts...)
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
	k8sCluster, err := linode.NewLkeCluster(ctx, label, &linode.LkeClusterArgs{
		K8sVersion: pulumi.String(args.K8sVersion),
		Label:      pulumi.String(label),
		Pools:      nodePools,
		ControlPlane: &linode.LkeClusterControlPlaneArgs{
			HighAvailability: pulumi.Bool(true),
		},
		Region: pulumi.String(args.Region),
	}, pulumi.Parent(&lkeResource))
	if err != nil {
		return nil, err
	}

	// decode and write kubeconfig
	// lkeResource.Cluster = k8sCluster
	// lkeResource.Label = label
	lkeResource.DecodeKubeconfig().WriteKubeconfig()

	// create provider
	provider, err := kubernetes.NewProvider(ctx, "lkeProvider", &kubernetes.ProviderArgs{
		Kubeconfig: lkeResource.Cluster.Kubeconfig.ToStringOutput(),
	}, pulumi.Parent(&lkeResource))
	if err != nil {
		msg := fmt.Sprintf("error instantiating kubernetes provider: %v", err)
		ctx.Log.Error(msg, nil)
	}

	lkeResource.Cluster = k8sCluster
	lkeResource.Label = label
	lkeResource.Provider = provider
	ctx.RegisterResourceOutputs(&lkeResource, pulumi.Map{
		"lkeCluster":           pulumi.ToOutput(k8sCluster),
		"lkeClusterLabel":      pulumi.ToOutput(label),
		"lkeClusterKubeconfig": pulumi.ToOutput(lkeResource.Kubeconfig),
	})

	return &lkeResource, nil
}
