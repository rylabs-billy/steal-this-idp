package main

import (
	"fmt"
	"time"

	"github.com/pulumi/pulumi-linode/sdk/v4/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"github.com/rylabs-billy/steal-this-idp/tree/refactor/micro-stacks/infra/app"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		var (
			nodePoolArray []*app.LinodeLkeNodePool
			cfg           = config.New(ctx, "linode")
			token         = cfg.Require("token")
			// objAccessKey  = cfg.Require("objAccessKey")
			// objSecretKey  = cfg.Require("objSecretKey")
			domainName = "demo.linodemarketplace.xyz"
			region     = "nl-ams"
			email      = "bthompso@akamai.com"
			objPrefix  = "apl"
			objBuckets = []string{
				"loki",
				"cnpg",
				"velero",
				"harbor",
				"thanos",
				"tempo",
				"gitea",
			}
			tags = []string{
				"marketplace",
				"apl",
				"dev",
			}
		)

		// linode: configure provider
		linodeProvider, _ := linode.NewProvider(ctx, "linodeProvider", &linode.ProviderArgs{
			ObjBucketForceDelete: pulumi.Bool(true),
			Token:                pulumi.String(token),
		})

		// obj: create a separate, region scoped key
		objkey, err := linode.NewObjectStorageKey(ctx, "pulumi-obj-key", &linode.ObjectStorageKeyArgs{
			Label: pulumi.String("pulumi-obj-key"),
			Regions: pulumi.StringArray{
				pulumi.String(region),
			},
		}, pulumi.Provider(linodeProvider))
		if err != nil {
			return err
		}

		// obj: provision buckets
		for _, bucket := range objBuckets {
			bucketName := fmt.Sprintf("%s-%s", objPrefix, bucket)
			_, err := app.NewLinodeObjBucket(ctx, bucketName, &app.LinodeObjBucketArgs{
				Key:    objkey,
				Region: region,
			}, pulumi.Provider(linodeProvider), pulumi.DependsOn([]pulumi.Resource{objkey}))
			if err != nil {
				return err
			}
			time.Sleep(1 * time.Second)
		}

		// lke: define the node pools
		initialNodePool := &app.LinodeLkeNodePool{
			Autoscale: true,
			NodeLabels: map[string]string{
				"platform":    "apl-demo",
				"environment": "dev",
			},
			Tags: tags,
			Type: "g6-dedicated-8",
		}
		nodePoolArray = append(nodePoolArray, initialNodePool)

		// lke: deploy the cluster
		aplcluster, err := app.NewLinodeLkeCluster(ctx, "apl-demo", &app.LinodeLkeClusterArgs{
			NodePools:          nodePoolArray,
			Region:             region,
			StaticLoadBalancer: true,
		}, pulumi.Provider(linodeProvider))
		if err != nil {
			return err
		}

		// lke: static nodebalancer
		loadbalancer, err := app.NewStaticLoadBalancer(ctx, "staticLoadBalancer", &app.StaticLoadBalancerArgs{
			Provider: aplcluster.Provider,
			Region:   region,
			Tag:      "apl-static-lb",
		}, pulumi.Provider(linodeProvider), pulumi.DependsOn([]pulumi.Resource{aplcluster}))
		if err != nil {
			return err
		} else {
			loadbalancer.Update()
		}

		// dns: create zone
		apldomain, err := app.NewLinodeDomain(ctx, domainName, &app.LinodeDomainArgs{
			Email: email,
			Tags:  tags,
		}, pulumi.Provider(linodeProvider), pulumi.DependsOn([]pulumi.Resource{aplcluster, loadbalancer}))
		if err != nil {
			return err
		} else {
			apldomain.Update().SetDefaultRecords(loadbalancer)
		}

		// stack outputs
		stack := fmt.Sprintf("bthompso/apl-demo/%s", ctx.Stack())
		ctx.Export("stack", pulumi.Map{
			"stackName": pulumi.ToOutput(stack),
			"region":    pulumi.ToOutput(region),
		})

		ctx.Export("domain", pulumi.Map{
			"id":   pulumi.ToOutput(apldomain.Id),
			"name": pulumi.ToOutput(apldomain.Name),
		})

		ctx.Export("kubecluster", pulumi.Map{
			"label":        pulumi.ToOutput(aplcluster.Label),
			"loadbalancer": pulumi.ToOutput(loadbalancer),
			"provider":     pulumi.ToOutput(aplcluster.Provider),
			"kubeconfig":   pulumi.ToOutput(aplcluster.Kubeconfig),
		})

		ctx.Export("obj", pulumi.Map{
			"accessKey": pulumi.ToOutput(objkey.AccessKey),
			"secretKey": pulumi.ToOutput(objkey.SecretKey),
			"prefix":    pulumi.ToOutput(objPrefix),
			"buckets":   pulumi.ToOutput(objBuckets),
		})

		return nil
	})
}
