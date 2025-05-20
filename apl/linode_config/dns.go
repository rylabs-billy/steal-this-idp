package linode_config

import (
	"fmt"
	"strings"

	utils "github.com/rylabs-billy/apl-demo/apl/internal"

	"github.com/pulumi/pulumi-linode/sdk/v4/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type LinodeDomain struct {
	pulumi.ResourceState

	DomainName pulumi.StringOutput `pulumi:"domainName"`
	DomainId   int                 `pulumi:"domainId"`
}

type LinodeDomainArgs struct {
	Email     string
	Ipv4      string
	Ipv6      string
	Subdomain string
	Ttl       int
	Type      string
}

type LinodeDomainRecord struct {
	Ipv4 string
	Ipv6 string
	Txt  string
	Name string
}

func DomainCreate(ctx *pulumi.Context, name string, email string) (*linode.Domain, error) {
	domain, err := linode.NewDomain(ctx, name, &linode.DomainArgs{
		Type:     pulumi.String("master"),
		Domain:   pulumi.String(name),
		SoaEmail: pulumi.String(email),
		TtlSec:   pulumi.Int(30),
	})
	if err != nil {
		return domain, err
	}

	return domain, err
}

func DomainRecord(ctx *pulumi.Context, id int, name string, recType string, target string) error {
	_, err := linode.NewDomainRecord(ctx, name, &linode.DomainRecordArgs{
		DomainId:   pulumi.Int(id),
		Name:       pulumi.String(name),
		RecordType: pulumi.String(recType),
		Target:     pulumi.String(target),
	})
	if err != nil {
		return err
	}
	return nil
}

func (d *LinodeDomainArgs) SetDefaults() {
	if d.Ttl == 0 {
		d.Ttl = 30
	}
	if d.Type == "" {
		d.Type = "master"
	}

}

func NewLinodeDomain(ctx *pulumi.Context, domainName string, domainArgs *LinodeDomainArgs, opts ...pulumi.ResourceOption) (*LinodeDomain, error) {
	var domainResource LinodeDomain
	args := domainArgs
	args.SetDefaults()

	err := ctx.RegisterComponentResource("pkg:index:LinodeDomain", domainName, &domainResource, opts...)
	if err != nil {
		return nil, err
	}

	_, err = linode.NewDomain(ctx, domainName, &linode.DomainArgs{
		Type:     pulumi.String(args.Type),
		Domain:   pulumi.String(domainName),
		SoaEmail: pulumi.String(args.Email),
		TtlSec:   pulumi.Int(args.Ttl),
	}, pulumi.Parent(&domainResource))
	if err != nil {
		msg := fmt.Sprintf("error creating linode domain: %v", err)
		ctx.Log.Error(msg, nil)
	}

	domainResource.DomainName = pulumi.String(domainName).ToStringOutput()
	return &domainResource, nil
}

func (d *LinodeDomain) Update(i utils.DomainSpec) {
	d.DomainId = i.Id
}

func (d *LinodeDomain) SetDefaultRecords(ctx *pulumi.Context, i utils.InfraResourceInfo) {
	dr := LinodeDomainRecord{
		Ipv4: i.NodeBalancer.Ipv4,
		Ipv6: i.NodeBalancer.Ipv6,
	}
	CreateDomainRecord(ctx, d.DomainId, dr)
}

func CreateDomainRecord(ctx *pulumi.Context, id int, record LinodeDomainRecord) error {
	check := func(err error) error {
		if err != nil {
			if !strings.Contains(err.Error(), "hostname and IP address already exists") {
				return err
			}
		}
		return nil
	}

	resourceName := func(t string) string {
		if record.Name != "" {
			if strings.Contains(record.Name, "-txt") {
				txtName := record.Name
				trimmed := strings.TrimSuffix(record.Name, "-txt")
				record.Name = trimmed
				return txtName
			}
			return record.Name
		}
		return t
	}

	mkRecord := func(ty string) error {
		var rtype, target string
		switch ty {
		case "ipv4":
			rtype = "A"
			target = record.Ipv4
		case "ipv6":
			rtype = "AAAA"
			target = record.Ipv6
		case "txt":
			rtype = "TXT"
			target = record.Txt
		}
		_, err := linode.NewDomainRecord(ctx, resourceName(ty), &linode.DomainRecordArgs{
			DomainId:   pulumi.Int(id),
			Name:       pulumi.String(record.Name),
			RecordType: pulumi.String(rtype),
			Target:     pulumi.String(target),
		})
		if err != nil {
			return err
		}
		return nil
	}

	if record.Ipv4 != "" {
		err := mkRecord("ipv4")
		return check(err)
	}
	if record.Ipv6 != "" {
		err := mkRecord("ipv6")
		return check(err)
	}
	if record.Txt != "" {
		err := mkRecord("txt")
		return check(err)
	}

	return nil
}

func AplDnsRecords(ctx *pulumi.Context, id int, ip string) {
	apps := []string{
		"alertmanager",
		"api",
		"argocd",
		"auth",
		"console",
		"drone",
		"gitea",
		"grafana",
		"harbor",
		"jaeger",
		"keycloak",
		"prometheus",
		"tekton",
		"tty",
	}

	teamApps := []string{
		"alertmanager-develop",
		"grafana-develop",
		"tekton-develop",
	}

	apps = append(apps, teamApps...)

	for _, app := range apps {
		appTxt1 := fmt.Sprintf("a-%s-txt", app)
		appTxt2 := fmt.Sprintf("%s-txt", app)
		extDns := "\"heritage=external-dns,external-dns/owner=default\""
		aRec := LinodeDomainRecord{
			Ipv4: ip,
			Name: app,
		}
		txtRec1 := LinodeDomainRecord{
			Name: appTxt1,
			Txt:  extDns,
		}
		txtRec2 := LinodeDomainRecord{
			Name: appTxt2,
			Txt:  extDns,
		}
		CreateDomainRecord(ctx, id, aRec)
		CreateDomainRecord(ctx, id, txtRec1)
		CreateDomainRecord(ctx, id, txtRec2)
	}
}
