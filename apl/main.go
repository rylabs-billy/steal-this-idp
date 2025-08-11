package main

import (
	"fmt"
	"strconv"
	"time"

	utils "github.com/rylabs-billy/apl-demo/apl/internal"
	"github.com/rylabs-billy/apl-demo/apl/k8s_config"
	"github.com/rylabs-billy/apl-demo/apl/linode_config"

	"github.com/pulumi/pulumi-linode/sdk/v4/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// set initial vars
		var (
			cfg           = config.New(ctx, "linode")
			aplcfg        = config.New(ctx, "apl")
			step          = config.Get(ctx, "step")
			token         = cfg.Require("token")
			objAccessKey  = cfg.Require("objAccessKey")
			objSecretKey  = cfg.Require("objSecretKey")
			otomiAdmin    = aplcfg.Require("otomiAdminPassword")
			teamDevelop   = aplcfg.Require("teamDevelopPassword")
			ageKey        = aplcfg.Require("agePublicKey")
			agePrivKey    = aplcfg.Require("agePrivateKey")
			lokiAdmin     = aplcfg.Require("lokiAdminPassword")
			aplVersion    = "4.8.0"
			domainName    = "demo.linodemarketplace.xyz"
			region        = "nl-ams"
			email         = "bthompso@akamai.com"
			nodePoolArray = []*linode_config.LinodeLkeNodePool{}
			objPrefix     = "apl"
			objBuckets    = []string{
				"loki",
				"cnpg",
				"velero",
				"harbor",
				"thanos",
				"tempo",
				"gitea",
			}
		)

		// module scoped resource vars
		var (
			aplcluster   *linode_config.LinodeLkeCluster
			loadbalancer *linode_config.StaticLoadBalancer
			infraInfo    utils.InfraResourceInfo
			apldomain    *linode_config.LinodeDomain
			dnsIsReady   utils.DnsIsReady
		)

		// configure linode provider
		linodeProvider, _ := linode.NewProvider(ctx, "linodeProvider", &linode.ProviderArgs{
			ObjBucketForceDelete: pulumi.Bool(true),
			Token:                pulumi.String(token),
		})

		if step >= "0" {
			//provision obj buckets
			objkey, err := linode.NewObjectStorageKey(ctx, "pulumi-obj-key", &linode.ObjectStorageKeyArgs{
				Label: pulumi.String("pulumi-obj-key"),
				Regions: pulumi.StringArray{
					pulumi.String(region),
				},
			}, pulumi.Provider(linodeProvider))
			if err != nil {
				return err
			}

			for _, bucket := range objBuckets {
				bucketName := fmt.Sprintf("%s-%s", objPrefix, bucket)
				_, err := linode_config.NewLinodeObjBucket(ctx, bucketName, &linode_config.LinodeObjBucketArgs{
					Key:    objkey,
					Region: region,
				}, pulumi.Provider(linodeProvider), pulumi.DependsOn([]pulumi.Resource{objkey}))
				if err != nil {
					return err
				}
				time.Sleep(1 * time.Second)
			}

			// deploy lke cluster
			initialNodePool := &linode_config.LinodeLkeNodePool{
				Autoscale: true,
				NodeLabels: map[string]string{
					"platform":    "apl-demo",
					"environment": "dev",
				},
				Tags: []string{
					"marketplace",
					"apl",
					"dev",
				},
				Type: "g6-dedicated-8",
			}
			nodePoolArray = append(nodePoolArray, initialNodePool)

			aplcluster, err = linode_config.NewLinodeLkeCluster(ctx, "apl-demo", &linode_config.LinodeLkeClusterArgs{
				NodePools:          nodePoolArray,
				Region:             region,
				StaticLoadBalancer: true,
			}, pulumi.Provider(linodeProvider))
			if err != nil {
				return err
			}

			// static nodebalancer
			loadbalancer, err = linode_config.NewStaticLoadBalancer(ctx, "staticLoadBalancer", &linode_config.StaticLoadBalancerArgs{
				Provider: aplcluster.LkeClusterProvider,
				Tag:      "apl-static-lb",
			}, pulumi.Provider(linodeProvider), pulumi.DependsOn([]pulumi.Resource{aplcluster}))
			if err != nil {
				return err
			}

			// creat dns zone
			apldomain, err = linode_config.NewLinodeDomain(ctx, domainName, &linode_config.LinodeDomainArgs{
				Email: email,
			}, pulumi.Provider(linodeProvider), pulumi.DependsOn([]pulumi.Resource{aplcluster, loadbalancer}))
			if err != nil {
				return err
			}
		}

		if step >= "1" && utils.AssertResource(aplcluster, apldomain, loadbalancer) {
			infraInfo.GetDomainInfo(ctx, domainName)
			infraInfo.GetNodeBalancerInfo(ctx, loadbalancer.Tag, region)
			if infraInfo.NodeBalancer.Id > 0 {
				loadbalancer.Update(ctx, &infraInfo).DeleteStaticService(ctx, aplcluster.LkeClusterLabel)
				aplcluster.StaticLoadBalancer(loadbalancer)

				if infraInfo.Domain.Id > 0 {
					apldomain.Update(infraInfo.Domain)
					apldomain.SetDefaultRecords(ctx, infraInfo)
					// check for removal
					linode_config.AplDnsRecords(ctx, infraInfo.Domain.Id, infraInfo.NodeBalancer.Ipv4)
				}
			}
		}

		if step >= "2" && utils.AssertResource(aplcluster, apldomain, infraInfo) {
			subdomains := []string{
				"auth",
				"keycloak",
			}

			for _, domain := range subdomains {
				resourceName := fmt.Sprintf("%sDnsReady", domain)
				fqdn := fmt.Sprintf("%s.%s", domain, domainName)
				_, err := utils.NewWaitForDns(ctx, resourceName, &utils.WaitForDnsArgs{
					Domain:  fqdn,
					Ip:      infraInfo.NodeBalancer.Ipv4,
					Timeout: 10,
				}, pulumi.Provider(linodeProvider), pulumi.DependsOn([]pulumi.Resource{aplcluster, apldomain}))
				if err != nil {
					return err
				}
				if domain == "auth" {
					dnsIsReady.Auth = true
				}
				if domain == "keycloak" {
					dnsIsReady.Keycloak = true
				}
			}
		}

		if step == "3" && utils.AssertResource(aplcluster, apldomain, infraInfo, dnsIsReady) {
			nbid := strconv.Itoa(infraInfo.NodeBalancer.Id)
			override := map[string]any{
				"region":           region,
				"domain":           domainName,
				"token":            token,
				"accessKey":        objAccessKey,
				"secretKey":        objSecretKey,
				"prefix":           objPrefix,
				"buckets":          objBuckets,
				"nodebalancerId":   nbid,
				"nodebalancerIpv4": infraInfo.NodeBalancer.Ipv4,
				"nodebalancerTag":  loadbalancer.Tag,
				"ageKey":           ageKey,
				"agePrivKey":       agePrivKey,
				"lokiAdmin":        lokiAdmin,
				"otomiAdmin":       otomiAdmin,
				"teamDevelop":      teamDevelop,
			}

			// deploy helm chart
			values := utils.YamlTemplate(ctx, "./helm/apl-values.tpl", override)
			aplChart := k8s_config.HelmOptions{
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

			_, err := k8s_config.NewKubeClusterConfig(ctx, "aplCluster", &k8s_config.KubeClusterConfigArgs{
				ClusterResource: aplcluster,
				HelmChart:       aplChart,
				LoadBalancer:    loadbalancer,
			}, pulumi.Provider(linodeProvider), pulumi.DependsOn([]pulumi.Resource{aplcluster, apldomain}), pulumi.DeletedWith(aplcluster))
			if err != nil {
				return err
			}
		}
		return nil
	})
}
