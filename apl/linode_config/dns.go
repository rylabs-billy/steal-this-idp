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
	Id         int                 `pulumi:"domainId"`
	Ctx        *pulumi.Context     `pulumi:"domainCtx"` // new
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

func (d *LinodeDomain) Update(ctx *pulumi.Context, i utils.DomainSpec) *LinodeDomain {
	d.Id = i.Id
	d.Ctx = ctx
	return d
}

func (d *LinodeDomain) SetDefaultRecords(i utils.InfraResourceInfo) *LinodeDomain {
	dr := LinodeDomainRecord{
		Ipv4: i.NodeBalancer.Ipv4,
		Ipv6: i.NodeBalancer.Ipv6,
	}
	CreateDomainRecord(d.Ctx, d.Id, dr, d)
	return d
}

func (d *LinodeDomain) AplDnsRecords(i utils.InfraResourceInfo, a []string) {
	for _, app := range a {
		aRec := LinodeDomainRecord{
			Ipv4: i.NodeBalancer.Ipv4,
			Name: app,
		}
		CreateDomainRecord(d.Ctx, d.Id, aRec, d)
	}
}

func CreateDomainRecord(ctx *pulumi.Context, id int, record LinodeDomainRecord, domain *LinodeDomain) error {
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
		}, pulumi.DeletedWith(domain))
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
