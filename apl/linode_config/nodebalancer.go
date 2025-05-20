package linode_config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	utils "github.com/rylabs-billy/apl-demo/apl/internal"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type StaticLoadBalancer struct {
	pulumi.ResourceState

	Label string `pulumi:"StaticLoadbalancer"`
	Tag   string `pulumi:"StaticLoadbalancerTag"`
	Id    int    `pulumi:"StaticLoadbalancerId"`
	Ipv4  string `pulumi:"StaticLoadbalancerIpv4"`
	Ipv6  string `pulumi:"StaticLoadbalancerIpv6"`
}

type StaticLoadBalancerArgs struct {
	Provider *kubernetes.Provider
	Label    string
	Tag      string
}

func (s *StaticLoadBalancerArgs) SetDefaults() {
	if s.Label == "" {
		s.Label = "StaticLoadBalancer"
	}
	if s.Tag == "" {
		s.Tag = "apl-static-lb"
	}
}

func (s *StaticLoadBalancer) Update(ctx *pulumi.Context, i *utils.InfraResourceInfo) *StaticLoadBalancer {
	s.Id = i.NodeBalancer.Id
	s.Ipv4 = i.NodeBalancer.Ipv4
	s.Ipv6 = i.NodeBalancer.Ipv6
	return s
}

func (s *StaticLoadBalancer) DeleteStaticService(ctx *pulumi.Context, k string) *StaticLoadBalancer {
	err := deleteStaticService(ctx, s.Label, k)
	if err != nil {
		return nil
	}
	return s
}

func deleteStaticService(ctx *pulumi.Context, svc string, kubecfg string) error {
	svcName := strings.ToLower(svc)
	cmdName := fmt.Sprintf("Delete%s", svc)

	fileName := fmt.Sprintf("%v-kubeconfig.yaml", kubecfg)
	homeDir := os.Getenv("HOME")
	kubeconfig := filepath.Join(homeDir, ".kube", fileName)
	cmd := fmt.Sprintf("export KUBECONFIG=%s; kubectl delete svc %s", kubeconfig, svcName)

	_, err := local.NewCommand(ctx, cmdName, &local.CommandArgs{
		Create: pulumi.String(cmd),
		Interpreter: pulumi.StringArray{
			pulumi.String("/bin/bash"),
			pulumi.String("-c"),
		},
	})
	if err != nil {
		if !strings.Contains(err.Error(), "Error from server (NotFound)") {
			return err
		}
	}
	return nil
}

func NewStaticLoadBalancer(ctx *pulumi.Context, loadbalancerName string, args *StaticLoadBalancerArgs, opts ...pulumi.ResourceOption) (*StaticLoadBalancer, error) {
	var loadbalancerResource StaticLoadBalancer
	provider := args.Provider
	args.SetDefaults()

	err := ctx.RegisterComponentResource("pkg:index:StaticLoadBalancer", loadbalancerName, &loadbalancerResource, opts...)
	if err != nil {
		return nil, err
	}

	svcName := strings.ToLower(args.Label)
	// create static loadbalancer service
	_, err = corev1.NewService(ctx, args.Label, &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Annotations: pulumi.StringMap{
				"service.beta.kubernetes.io/linode-loadbalancer-tags":     pulumi.String(args.Tag),
				"service.beta.kubernetes.io/linode-loadbalancer-preserve": pulumi.String("true"),
			},
			DeletionGracePeriodSeconds: pulumi.Int(10),
			Name:                       pulumi.String(svcName),
			Namespace:                  pulumi.String("default"),
		},
		Spec: &corev1.ServiceSpecArgs{
			Type: pulumi.String("LoadBalancer"),
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{
					Name:       pulumi.String("http"),
					Port:       pulumi.Int(80),
					Protocol:   pulumi.String("TCP"),
					TargetPort: pulumi.Any("http"),
				},
				&corev1.ServicePortArgs{
					Name:       pulumi.String("https"),
					Port:       pulumi.Int(443),
					Protocol:   pulumi.String("TCP"),
					TargetPort: pulumi.Any("https"),
				},
			},
		},
	}, pulumi.Provider(provider), pulumi.Parent(&loadbalancerResource))
	if err != nil {
		return nil, err
	}

	loadbalancerResource.Label = args.Label
	loadbalancerResource.Tag = args.Tag

	return &loadbalancerResource, nil
}
