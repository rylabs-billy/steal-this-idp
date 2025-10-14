package app

import (
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi-linode/sdk/v4/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type DnsRecord struct {
	Domain       *linode.Domain
	Name         string
	Opts         PulumiOpts
	RecType      string
	ResourceName string
	Tag          string
	Target       string
	Ttl          int
}

func (d *DnsRecord) SetDefaults() {
	if d.ResourceName == "" {
		if d.Name == "" {
			switch d.RecType {
			case "A":
				d.ResourceName = "defaultIpv4"
			case "AAAA":
				d.ResourceName = "defaultIpv6"
			}
		} else {
			d.ResourceName = d.Name
		}
	}
	if d.Ttl < 30 {
		d.Ttl = 30
	}
}

func GetDomainId(d *linode.Domain) (pulumi.IntOutput, bool) {
	domainId, ok := d.ID().ApplyT(func(i string) (int, error) {
		id, err := strconv.Atoi(i)
		if err != nil {
			return 0, err
		}
		return id, nil
	}).(pulumi.IntOutput)

	return domainId, ok
}

func AddDnsRecord(ctx *pulumi.Context, d DnsRecord) error {
	d.SetDefaults()
	id, ok := GetDomainId(d.Domain)
	if ok {
		_, err := linode.NewDomainRecord(ctx, d.ResourceName, &linode.DomainRecordArgs{
			DomainId:   id,
			Name:       pulumi.String(d.Name),
			RecordType: pulumi.String(d.RecType),
			Tag:        pulumi.String(d.Tag),
			Target:     pulumi.String(d.Target),
			TtlSec:     pulumi.Int(d.Ttl),
		}, pulumi.DependsOn(d.Opts.DependsOn), pulumi.DeletedWith(d.Domain))
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("error: unable to get domain ID")
}
