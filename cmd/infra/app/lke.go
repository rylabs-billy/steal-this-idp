package app

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	utils "github.com/rylabs-billy/steal-this-idp/utils"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"gopkg.in/yaml.v2"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-linode/sdk/v4/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type AclAddr struct {
	Ipv4, Ipv6 []string
}

type ControlPlane struct {
	Acl, HA bool
	Addrs   []AclAddr
}

type LkeProvider struct {
	pulumi.ResourceState

	Provider *kubernetes.Provider `pulumi:"lkeProvider"`
}

type LkeProviderArgs struct {
	Cluster *linode.LkeCluster
	Label   string
}

type NodePool struct {
	Autoscaler bool
	Count      int
	Labels     map[string]string
	Max        int
	Tags       []string
	Type       string
}

type StaticLoadbalancer struct {
	pulumi.ResourceState

	Id    string              `pulumi:"StaticLoadbalancer"`
	Ipv4  string              `pulumi:"StaticLoadbalancerIpv4"`
	Ipv6  string              `pulumi:"StaticLoadbalancerIpv6"`
	Label pulumi.StringOutput `pulumi:"StaticLoadbalancer"`
}

type StaticLoadbalancerArgs struct {
	Annotations map[string]string
	Label       string
	Kubecfg     string
	Region      string
}

type KubeSvc struct {
	Args   *StaticLoadbalancerArgs
	Name   string
	Parent *StaticLoadbalancer
}

func (np *NodePool) SetDefaults() {
	if np.Autoscaler {
		if np.Max == 0 {
			np.Max = 15
		}
	}

	if np.Count == 0 {
		np.Count = 3
	}

	if np.Type == "" {
		np.Type = "g6-dedicated-8"
	}
}

func lkeNodePool(np NodePool) linode.LkeClusterPoolArgs {
	var (
		autoscale linode.LkeClusterPoolAutoscalerArgs
		nodeTags  pulumi.StringArray
	)
	nodeLabels := pulumi.StringMap{}

	if np.Autoscaler {
		autoscale = linode.LkeClusterPoolAutoscalerArgs{
			Max: pulumi.Int(np.Max),
			Min: pulumi.Int(np.Count),
		}
	}

	if utils.AssertResource(np.Labels) {
		for k, v := range np.Labels {
			nodeLabels[k] = pulumi.String(v)
		}
	}

	if utils.AssertResource(np.Tags) {
		for _, i := range np.Tags {
			nodeTags = append(nodeTags, pulumi.String(i))
		}
	}

	nodePool := linode.LkeClusterPoolArgs{
		Type:       pulumi.String(np.Type),
		Autoscaler: autoscale,
		Count:      pulumi.Int(np.Count),
		Labels:     nodeLabels,
		Tags:       nodeTags,
	}

	return nodePool
}

func lkeControlPlane(cp ControlPlane) linode.LkeClusterControlPlaneArgs {
	var (
		aclArgs linode.LkeClusterControlPlaneAclArgs
		addrs   linode.LkeClusterControlPlaneAclAddressArray
	)

	ipArray := func(f []string) pulumi.StringArray {
		var ips pulumi.StringArray
		for _, i := range f {
			ip := net.ParseIP(i)
			if ip != nil {
				ips = append(ips, pulumi.String(i))
			}
		}

		return ips
	}

	if cp.Acl {
		for _, i := range cp.Addrs {
			// AddressArgs fields
			var addrArgs linode.LkeClusterControlPlaneAclAddressArgs

			if utils.AssertResource(i.Ipv4) {
				addrArgs.Ipv4s = ipArray(i.Ipv4)
			}

			if utils.AssertResource(i.Ipv6) {
				addrArgs.Ipv4s = ipArray(i.Ipv4)
			}
			addrs = append(addrs, addrArgs)
		}

		aclArgs.Addresses = addrs
		aclArgs.Enabled = pulumi.Bool(true)
	}

	controlPlane := linode.LkeClusterControlPlaneArgs{
		Acl:              aclArgs,
		HighAvailability: pulumi.Bool(cp.HA),
	}

	return controlPlane
}

func decodeKubeconfig(ctx *pulumi.Context, c *linode.LkeCluster, label string, write bool) pulumi.StringOutput {
	dec := c.Kubeconfig.ApplyT(func(k string) string {
		d, err := base64.StdEncoding.DecodeString(string(k))
		if err != nil {
			msg := fmt.Sprintf("error decoding kubeconfig: %v", err)
			ctx.Log.Error(msg, nil)
		}

		// write kubeconfig to filesystem
		if write {
			var data map[string]interface{}
			err := yaml.Unmarshal(d, &data)
			if err != nil {
				msg := fmt.Sprintf("error with yaml unmarshal: %v", err)
				ctx.Log.Error(msg, nil)
			}

			homeDir := os.Getenv("HOME")
			fileName := fmt.Sprintf("%s-kubeconfig.yaml", label)
			file := filepath.Join(homeDir, ".kube", fileName)

			err = utils.WriteFile(file, d)
			if err != nil {
				msg := fmt.Sprintf("error writing file to os: %v", err)
				ctx.Log.Error(msg, nil)
			}
		}

		return string(d)
	}).(pulumi.StringOutput)

	return dec
}

// need this for now, but I don't like it, and will soon refactor
func NewLkeProvider(ctx *pulumi.Context, providerName string, args *LkeProviderArgs, opts ...pulumi.ResourceOption) (*LkeProvider, error) {
	var lkeProviderResource LkeProvider

	err := ctx.RegisterComponentResource("pkg:index:LkeProvider", providerName, &lkeProviderResource, opts...)
	if err != nil {
		return nil, err
	}

	kubecfg := decodeKubeconfig(ctx, args.Cluster, args.Label, true)
	provider, err := kubernetes.NewProvider(ctx, "lkeProvider", &kubernetes.ProviderArgs{
		Kubeconfig: kubecfg,
	}, pulumi.Parent(&lkeProviderResource))
	if err != nil {
		return nil, err
	}

	lkeProviderResource.Provider = provider

	// ctx.Export("lkeProvider", provider)
	ctx.RegisterResourceOutputs(&lkeProviderResource, pulumi.Map{
		"lkeProvider": provider,
	})

	return &lkeProviderResource, nil
}

func NewStaticLoadbalancer(ctx *pulumi.Context, loadbalancerName string, args *StaticLoadbalancerArgs, opts ...pulumi.ResourceOption) (*StaticLoadbalancer, error) {
	var loadbalancerResource StaticLoadbalancer
	region := args.Region

	// default label if none was provided
	if args.Label == "" {
		args.Label = "StaticLoadbalancer"
	}

	err := ctx.RegisterComponentResource("pkg:index:StaticLoadbalancer", loadbalancerName, &loadbalancerResource, opts...)
	if err != nil {
		return nil, err
	}

	// create static loadbalancer service
	svcName := strings.ToLower(args.Label)
	svc := KubeSvc{
		Args:   args,
		Name:   svcName,
		Parent: &loadbalancerResource,
	}

	// only proceed if a matching nodebalancer does not already exist
	nbid, _ := ctx.GetConfig("nodebalancer-id")
	nb := GetNodeBalancer(ctx, region, nbTag)

	err = KubeService(ctx, svc, &loadbalancerResource)
	if err != nil {
		return nil, err
	}

	// get nodebalancer (loadbalancer info)
	nb = GetNodeBalancer(ctx, region, nbTag)
	id := strconv.Itoa(nb.id)

	// log to stdout if nodebalancer config has drifted
	if id != nbid {
		msg := fmt.Sprintf("\n[debug] nodebalancer drift\nwant %s, got %s", id, nbid)
		ctx.Log.Debug(msg, &pulumi.LogArgs{
			Resource:  &loadbalancerResource,
			Ephemeral: true,
		})
	}

	ctx.Export("loadbalancerId", pulumi.String(id))
	ctx.Export("ipv4", pulumi.String(nb.ipv4))
	ctx.Export("ipv6", pulumi.String(nb.ipv6))

	loadbalancerResource.Ipv4 = nb.ipv4
	loadbalancerResource.Ipv6 = nb.ipv6
	loadbalancerResource.Id = id
	loadbalancerResource.Label = pulumi.String(args.Label).ToStringOutput()

	ctx.RegisterResourceOutputs(&loadbalancerResource, pulumi.Map{
		"StaticLoadbalancer": pulumi.String(args.Label),
	})

	return &loadbalancerResource, nil
}

func KubeService(ctx *pulumi.Context, svc KubeSvc, p *StaticLoadbalancer) error {
	fileName := fmt.Sprintf("%v-kubeconfig.yaml", svc.Args.Kubecfg)
	homeDir := os.Getenv("HOME")
	kubeconfig := filepath.Join(homeDir, ".kube", fileName)

	// create
	// note: look into refactoring this with pulumi resource hooks
	// https://www.pulumi.com/docs/iac/concepts/options/hooks/
	cmd := fmt.Sprintf("export KUBECONFIG=%s; kubectl create svc loadbalancer %s --tcp=80,443", kubeconfig, svc.Name)
	createCmd, err := local.NewCommand(ctx, "createService", &local.CommandArgs{
		Create: pulumi.String(cmd),
		Interpreter: pulumi.StringArray{
			pulumi.String("/bin/bash"),
			pulumi.String("-c"),
		},
		Triggers: pulumi.Array{
			pulumi.String(kubeconfig),
		},
	}, pulumi.Parent(p))
	if err != nil {
		return err
	}

	// annotate
	var annotateCmd *local.Command
	count := -1
	for k, v := range svc.Args.Annotations {
		count++
		rname := fmt.Sprintf("Annotate-%s-%d", svc.Name, count)
		cmd := fmt.Sprintf("export KUBECONFIG=%s; kubectl annotate svc %s %s=%s", kubeconfig, svc.Name, k, v)
		annotateCmd, err = local.NewCommand(ctx, rname, &local.CommandArgs{
			Create: pulumi.String(cmd),
			Interpreter: pulumi.StringArray{
				pulumi.String("/bin/bash"),
				pulumi.String("-c"),
			},
			Triggers: pulumi.Array{
				pulumi.NewResourceOutput(createCmd),
			},
		}, pulumi.DependsOn([]pulumi.Resource{createCmd}), pulumi.IgnoreChanges([]string{"create"}), pulumi.Parent(p))
		if err != nil {
			return err
		}
	}

	// delete
	cmd = fmt.Sprintf("export KUBECONFIG=%s; kubectl delete svc %s", kubeconfig, svc.Name)
	_, err = local.NewCommand(ctx, "deleteService", &local.CommandArgs{
		Create: pulumi.String(cmd),
		Interpreter: pulumi.StringArray{
			pulumi.String("/bin/bash"),
			pulumi.String("-c"),
		},
		Triggers: pulumi.Array{
			pulumi.NewResourceOutput(annotateCmd),
		},
	}, pulumi.DependsOn([]pulumi.Resource{annotateCmd}), pulumi.Parent(p))
	if err != nil {
		return err
	}

	return nil
}
