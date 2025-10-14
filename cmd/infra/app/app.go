package app

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-linode/sdk/v4/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	utils "github.com/rylabs-billy/steal-this-idp/utils"
)

const (
	label      = "apl-demo"
	k8sVersion = "1.33"
	domainName = "demo.billythompson.io"
	region     = "nl-ams"
	email      = "bthompso@akamai.com"
	objPrefix  = "apl"
	nbLabel    = "StaticLoadbalancer"
	nbTag      = "apl-static-lb"
	stack      = "dev"
)

type PulumiResourceInfo struct {
	Resources map[string]interface{}
	Token     string
}

type PulumiOpts struct {
	DependsOn   []pulumi.Resource
	DeletedWith pulumi.Resource
}

func (r *PulumiResourceInfo) Build(ctx *pulumi.Context) error {
	err := build(ctx, r)
	return err
}

func (r *PulumiResourceInfo) Config(ctx *pulumi.Context) error {
	err := conf(ctx, r)
	return err
}

func build(ctx *pulumi.Context, r *PulumiResourceInfo) error {
	// cloud infra build func
	objBuckets := []string{
		"loki",
		"cnpg",
		"velero",
		"harbor",
		"thanos",
		"tempo",
		"gitea",
	}
	nodeLabels := map[string]string{
		"platform":    label,
		"environment": "dev",
	}
	tags := []string{
		"marketplace",
		"apl",
		"dev",
	}

	// linode: configure provider
	linodeProvider, _ := linode.NewProvider(ctx, "linodeProvider", &linode.ProviderArgs{
		ObjBucketForceDelete: pulumi.Bool(true),
		Token:                pulumi.String(r.Token),
	})
	r.Resources["linodeProvider"] = linodeProvider

	// obj: create a separate, region scoped key
	objkey, err := linode.NewObjectStorageKey(ctx, "pulumi-obj-key", &linode.ObjectStorageKeyArgs{
		Label: pulumi.String("pulumi-obj-key"),
		Regions: pulumi.StringArray{
			pulumi.String(region),
		},
	})
	if err != nil {
		return err
	}
	ctx.Export("obj", pulumi.StringMap{
		"objPrefix": pulumi.String(objPrefix),
		"accessKey": objkey.AccessKey,
		"secretKey": objkey.SecretKey,
	})

	// obj: provision buckets
	for _, bucket := range objBuckets {
		bucketName := fmt.Sprintf("%s-%s", objPrefix, bucket)
		_, err = linode.NewObjectStorageBucket(ctx, bucketName, &linode.ObjectStorageBucketArgs{
			AccessKey:      objkey.AccessKey,
			SecretKey:      objkey.SecretKey,
			Region:         pulumi.String(region),
			Label:          pulumi.String(bucketName),
			LifecycleRules: defaultLifecyclePolicy(),
		}, pulumi.DependsOn([]pulumi.Resource{objkey}))
		if err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}
	objBucketExp := utils.BuildPulumiStringArray(objBuckets)
	ctx.Export("objBuckets", objBucketExp)

	// dns: create zone
	tagArray := utils.BuildPulumiStringArray(tags)
	domain, err := linode.NewDomain(ctx, domainName, &linode.DomainArgs{
		Type:     pulumi.String("master"),
		Domain:   pulumi.String(domainName),
		SoaEmail: pulumi.String(email),
		Tags:     tagArray,
		TtlSec:   pulumi.Int(30),
	})
	if err != nil {
		return err
	}

	// dns: caa for subdomain wildcard certificate
	_ = domain.ID().ApplyT(func(i string) int {
		id, _ := strconv.Atoi(i)
		_, err = linode.NewDomainRecord(ctx, "wildcardCAA", &linode.DomainRecordArgs{
			DomainId:   pulumi.Int(id),
			RecordType: pulumi.String("CAA"),
			Target:     pulumi.String("letsencrypt.org"),
			Name:       pulumi.String(""),
			Tag:        pulumi.String("issuewild"),
			TtlSec:     pulumi.Int(30),
		}, pulumi.DeletedWith(domain))
		return id
	})
	r.Resources["domain"] = domain
	ctx.Export("domainName", pulumi.String(domainName))
	ctx.Export("domainId", domain.ID())

	// lke: configure node pools and control plane options
	np := NodePool{
		Autoscaler: true,
		Labels:     nodeLabels,
		Tags:       tags,
	}
	np.SetDefaults()
	aplNodePool := lkeNodePool(np)

	cp := ControlPlane{
		HA: true,
	}
	aplControlPlane := lkeControlPlane(cp)

	// lke: deploy kubernetes cluster
	aplcluster, err := linode.NewLkeCluster(ctx, label, &linode.LkeClusterArgs{
		K8sVersion:   pulumi.String(k8sVersion),
		Label:        pulumi.String(label),
		Pools:        linode.LkeClusterPoolArray{aplNodePool},
		Region:       pulumi.String(region),
		ControlPlane: aplControlPlane,
		Tags:         utils.BuildPulumiStringArray(tags),
	})
	if err != nil {
		return err
	}
	r.Resources["aplcluster"] = aplcluster
	ctx.Export("kubeconfig", aplcluster.Kubeconfig)
	ctx.Export("lkeClusterId", aplcluster.ID())

	//lke: create k8s provider
	lkeProvider, err := NewLkeProvider(ctx, "lkeProvider", &LkeProviderArgs{
		Cluster: aplcluster,
		Label:   label,
	}, pulumi.DependsOn([]pulumi.Resource{aplcluster}))
	if err != nil {
		return err
	}
	r.Resources["lkeProvider"] = lkeProvider.Provider

	return nil
}

func conf(ctx *pulumi.Context, r *PulumiResourceInfo) error {
	// cloud infra config func
	domain := r.Resources["domain"].(*linode.Domain)
	lke := r.Resources["aplcluster"].(*linode.LkeCluster)
	lkepv := r.Resources["lkeProvider"].(*kubernetes.Provider)

	// lke: use provider lookup func to get kube cluster id
	_, ok := lke.ID().ApplyT(func(i string) (int, error) {
		id, _ := strconv.Atoi(i)
		res, err := linode.LookupLkeCluster(ctx, &linode.LookupLkeClusterArgs{
			Id: id,
		})
		if err != nil {
			return 0, nil
		}

		retry := 0
		for range 3 {
			if res.Status == "ready" {
				break
			}
			time.Sleep(5 * time.Second)
			retry++
		}
		if res.Status != "ready" && retry == 3 {
			return 0, fmt.Errorf("error: timeout waiting for lke cluster after %d tries", retry)
		}

		return id, nil
	}).(pulumi.IntOutput)

	if ok {
		// lke: provision static loadbalancer (linode nodebalancer)
		annotations := map[string]string{
			"service.beta.kubernetes.io/linode-loadbalancer-tags":     nbTag,
			"service.beta.kubernetes.io/linode-loadbalancer-preserve": "true",
		}

		// lke: deploy a static loadbalancer to the cluster
		provider := r.Resources["linodeProvider"].(*linode.Provider)
		loadbalancer, err := NewStaticLoadbalancer(ctx, nbLabel, &StaticLoadbalancerArgs{
			Annotations: annotations,
			Label:       nbLabel,
			Kubecfg:     label,
			Provider:    provider, // use secondary linode provider to avoid lookup issues
		}, pulumi.DependsOn([]pulumi.Resource{lke, lkepv}), pulumi.DeletedWith(domain))
		if err != nil {
			return err
		}
		r.Resources["loadbalancer"] = loadbalancer
		ctx.Export(nbTag, loadbalancer.URN())
	}

	// dns: set default dns records for loadbalancer
	lb := r.Resources["loadbalancer"].(*StaticLoadbalancer)
	dnsOpts := PulumiOpts{DependsOn: []pulumi.Resource{lb}}
	dnsRec := func(ip, name, typ string) DnsRecord {
		return DnsRecord{Domain: domain, Opts: dnsOpts, Name: name, RecType: typ, Target: ip}
	}
	subdomains := map[string]string{
		"auth":     fmt.Sprintf("auth.%s", domainName),
		"keycloak": fmt.Sprintf("keycloak.%s", domainName),
		"api":      fmt.Sprintf("api.%s", domainName),
	}

	// dns: root domain ipv4 record
	if lb.Ipv4 != "" {
		err := AddDnsRecord(ctx, dnsRec(lb.Ipv4, "", "A"))
		if err != nil {
			return err
		}

		// dns: subdomains
		for k := range subdomains {
			err = AddDnsRecord(ctx, dnsRec(lb.Ipv4, k, "A"))
		}
		if err != nil {
			return err
		}
	}

	// dns: root domain ipv6 record
	if lb.Ipv6 != "" {
		err := AddDnsRecord(ctx, dnsRec(lb.Ipv6, "", "AAAA"))
		if err != nil {
			return err
		}
	}

	// export any remaining outputs we might want for the next stack
	exportVars := map[string]interface{}{
		"loadbalancerTag": pulumi.String("apl-static-lb"),
		"region":          pulumi.String(region),
		"aplDemoLabel":    pulumi.String(label),
		"subdomains":      utils.BuildPulumiStringMap(subdomains),
	}

	for k, i := range exportVars {
		switch v := i.(type) {
		case string:
			ctx.Export(k, pulumi.String(v))
		default:
			ctx.Export(k, pulumi.ToOutput(v))
		}
	}

	return nil
}
