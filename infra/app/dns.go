package app

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-linode/sdk/v4/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type LinodeDomain struct {
	pulumi.ResourceState

	Ctx  *pulumi.Context `pulumi:"domainCtx"`
	Id   int             `pulumi:"domainId"`
	Name string          `pulumi:"domainName"`
	Tags []string        `pulumi:"domainTags"`
}

type LinodeDomainArgs struct {
	Email     string
	Ipv4      string
	Ipv6      string
	Subdomain string
	Tags      []string
	Ttl       int
	Type      string
}

type LinodeDomainRecord struct {
	Ipv4 string
	Ipv6 string
	Name string
	Txt  string
}

func (d *LinodeDomainArgs) SetDefaults() {
	if d.Ttl == 0 {
		d.Ttl = 30
	}
	if d.Type == "" {
		d.Type = "master"
	}
}

func (d *LinodeDomain) Update() *LinodeDomain {
	d.Id = getDomainId(d.Ctx, d.Name, d.Tags)
	return d
}

func (d *LinodeDomain) SetDefaultRecords(n *StaticLoadBalancer) *LinodeDomain {
	dr := LinodeDomainRecord{
		Ipv4: n.Ipv4,
		Ipv6: n.Ipv6,
	}
	createDomainRecord(d.Ctx, d.Id, dr, d)
	return d
}

func (d *LinodeDomain) AplDnsRecords(n *StaticLoadBalancer, a []string) {
	for _, app := range a {
		aRec := LinodeDomainRecord{
			Ipv4: n.Ipv4,
			Name: app,
		}
		createDomainRecord(d.Ctx, d.Id, aRec, d)
	}
}

func getDomainId(ctx *pulumi.Context, name string, tags []string) int {
	matchMethod := "exact"
	res, err := linode.GetDomains(ctx, &linode.GetDomainsArgs{
		Filters: []linode.GetDomainsFilter{
			{
				Name: "domain",
				Values: []string{
					name,
				},
				MatchBy: &matchMethod,
			},
			{
				Name:    "tags",
				Values:  tags,
				MatchBy: &matchMethod,
			},
		},
	}, nil)

	if err != nil {
		msg := fmt.Sprintf("error finding domain: %v", err)
		ctx.Log.Error(msg, nil)
	}

	if len(res.Domains) == 0 {
		msg := fmt.Sprintf("search result: %v", res.Domains)
		ctx.Log.Error(msg, nil)
	}
	id := *res.Domains[0].Id
	fmt.Printf("The domain id is: %v", id)

	return id
}

func createDomainRecord(ctx *pulumi.Context, id int, record LinodeDomainRecord, domain *LinodeDomain) error {
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

func NewLinodeDomain(ctx *pulumi.Context, domainName string, domainArgs *LinodeDomainArgs, opts ...pulumi.ResourceOption) (*LinodeDomain, error) {
	var domainResource LinodeDomain
	args := domainArgs
	args.SetDefaults()
	tags := pulumi.StringArray{}

	if len(args.Tags) > 0 {
		for _, tag := range args.Tags {
			t := pulumi.String(tag)
			tags = append(tags, t)
		}
	}

	err := ctx.RegisterComponentResource("pkg:index:LinodeDomain", domainName, &domainResource, opts...)
	if err != nil {
		return nil, err
	}

	_, err = linode.NewDomain(ctx, domainName, &linode.DomainArgs{
		Type:     pulumi.String(args.Type),
		Domain:   pulumi.String(domainName),
		SoaEmail: pulumi.String(args.Email),
		Tags:     tags,
		TtlSec:   pulumi.Int(args.Ttl),
	}, pulumi.Parent(&domainResource))
	if err != nil {
		msg := fmt.Sprintf("error creating linode domain: %v", err)
		ctx.Log.Error(msg, nil)
	}

	domainResource.Name = domainName
	domainResource.Ctx = ctx
	domainResource.Tags = args.Tags

	return &domainResource, nil
}
