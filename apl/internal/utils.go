package internal

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/pulumi/pulumi-linode/sdk/v4/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"gopkg.in/yaml.v2"
)

type WaitForUrl struct {
	pulumi.ResourceState

	Name       pulumi.StringOutput `pulumi:"url"`
	StatusCode pulumi.IntOutput    `pulumi:"statusCode"`
}

type WaitForUrlArgs struct {
	Timeout int
	Retry   int
	Status  int
}

type WaitForDns struct {
	pulumi.ResourceState

	IsReady bool `pulumi:"dnsReady"`
}

type WaitForDnsArgs struct {
	Domain  string
	Ip      string
	Timeout int
}

type DnsIsReady struct {
	Auth     bool
	Keycloak bool
}

type InfraResourceInfo struct {
	Domain       DomainSpec
	NodeBalancer NodeBalancerSpec
}

type DomainSpec struct {
	Id int
}

type NodeBalancerSpec struct {
	Id   int
	Ipv4 string
	Ipv6 string
}

func (i *InfraResourceInfo) GetDomainInfo(ctx *pulumi.Context, query string) {
	i.Domain = getDomainInfo(ctx, query)
}

func (i *InfraResourceInfo) GetNodeBalancerInfo(ctx *pulumi.Context, query string, region string) {
	i.NodeBalancer = getNodeBalancerInfo(ctx, query, region)
}

func getDomainInfo(ctx *pulumi.Context, query string) DomainSpec {
	var domain DomainSpec
	matchMethod := "exact"
	result, err := linode.GetDomains(ctx, &linode.GetDomainsArgs{
		Filters: []linode.GetDomainsFilter{
			{
				Name: "domain",
				Values: []string{
					query,
				},
				MatchBy: &matchMethod,
			},
		},
	}, nil)
	if err != nil {
		msg := fmt.Sprintf("error finding domain: %v", err)
		ctx.Log.Error(msg, nil)
	}
	if len(result.Domains) > 0 {
		domain = DomainSpec{Id: *result.Domains[0].Id}
	}
	return domain
}

func getNodeBalancerInfo(ctx *pulumi.Context, query string, region string) NodeBalancerSpec {
	var nb NodeBalancerSpec
	matchMethod := "exact"
	result, err := linode.GetNodebalancers(ctx, &linode.GetNodebalancersArgs{
		Filters: []linode.GetNodebalancersFilter{
			{
				Name: "tags",
				Values: []string{
					query,
				},
				MatchBy: &matchMethod,
			},
			{
				Name: "region",
				Values: []string{
					region,
				},
				MatchBy: &matchMethod,
			},
		},
	}, nil)
	if err != nil {
		msg := fmt.Sprintf("error finding nodebalancer: %v", err)
		ctx.Log.Error(msg, nil)
	}
	if len(result.Nodebalancers) > 0 {
		nb = NodeBalancerSpec{
			Id:   result.Nodebalancers[0].Id,
			Ipv4: result.Nodebalancers[0].Ipv4,
			Ipv6: result.Nodebalancers[0].Ipv6,
		}
	}
	return nb
}

func DecodeString(s interface{}) string {
	switch t := s.(type) {
	case pulumi.StringOutput:
		var a = t.ApplyT(func(v string) string {
			dec, _ := base64.StdEncoding.DecodeString(v)
			return string(dec)
		}).ElementType().String()
		return a
	case string:
		dec, _ := base64.StdEncoding.DecodeString(t)
		return string(dec)
	default:
		return ""
	}
}

func YamlTemplate(ctx *pulumi.Context, tpl string, values map[string]any, write ...bool) string {
	t, err := template.ParseFiles(tpl)
	if err != nil {
		msg := fmt.Sprintf("error parsing values template file: %v", err)
		ctx.Log.Error(msg, nil)
	}

	buf := &bytes.Buffer{}
	t.Execute(buf, values)

	if write != nil && write[0] {
		var data map[string]any
		yaml.Unmarshal(buf.Bytes(), &data)

		yamlFile := strings.Replace(tpl, ".tpl", ".yaml", 1)
		WriteFile(yamlFile, buf.Bytes())
	}

	return buf.String()
}

func WriteFile(file string, data []byte) error {
	dir, _ := filepath.Split(file)

	if _, err := os.Stat(dir); !os.IsExist(err) {
		os.Mkdir(dir, 0755)
	} else {
		return err
	}

	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = io.Writer.Write(f, data); err != nil {
		return err
	}
	return f.Sync()
}

func DecodeKubeconfig(cluster *linode.LkeCluster, writeConfig bool) pulumi.StringOutput {
	decodedKubeconfig := cluster.Kubeconfig.ApplyT(func(k string) (string, error) {
		decoded, err := base64.StdEncoding.DecodeString(k)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}).(pulumi.StringOutput)

	if writeConfig {
		_ = cluster.Label.ApplyT(func(k string) error {
			WriteKubeConfig(decodedKubeconfig, k)
			return nil
		})
	}
	return decodedKubeconfig
}

func WriteKubeConfig(config pulumi.StringOutput, name string) {
	config.ApplyT(func(k string) error {
		var data map[string]interface{}
		yaml.Unmarshal([]byte(k), &data)

		homeDir := os.Getenv("HOME")
		fileName := fmt.Sprintf("%s-kubeconfig.yaml", name)
		file := filepath.Join(homeDir, ".kube", fileName)

		err := WriteFile(file, []byte(k))
		if err != nil {
			return err
		}
		return nil
	})
}

func ResolveDns(domain string, ip string, timeout int) (string, error) {
	var addr string
	start := time.Now()
	end := time.Duration(timeout) * time.Minute
	check := func(err error, t int) error {
		conditions := []bool{
			!strings.Contains(err.Error(), "no such host"),
			!strings.Contains(err.Error(), "server misbehaving"),
		}
		if conditions[0] && conditions[1] {
			return err
		}
		return nil
	}

	resolver := func(ctx context.Context, proto string, address string) (net.Conn, error) {
		d := net.Dialer{}
		return d.DialContext(ctx, "udp", "8.8.8.8:53")
	}

	dns := &net.Resolver{
		PreferGo: true,
		Dial:     resolver,
	}

	for time.Since(start) < end && addr != ip {
		result, err := dns.LookupIPAddr(context.Background(), domain)
		if err != nil {
			return "", check(err, timeout)
		}

		if len(result) > 0 {
			addr = result[0].IP.String()
		}
		time.Sleep(5 * time.Second)
	}

	if time.Since(start) >= end {
		return addr, fmt.Errorf("exceded timeout waiting for %s to resolve", domain)
	}

	return addr, nil
}

func NewWaitForDns(ctx *pulumi.Context, name string, args *WaitForDnsArgs, opts ...pulumi.ResourceOption) (*WaitForDns, error) {
	// resource wrapper around the ResolveDns func
	var dnsresource WaitForDns
	err := ctx.RegisterComponentResource("pkg:index:WaitForDns", name, &dnsresource, opts...)
	if err != nil {
		return nil, err
	}

	ipAddr, err := ResolveDns(args.Domain, args.Ip, args.Timeout)
	if err != nil {
		return nil, err
	}
	if ipAddr != "" {
		msg := fmt.Sprintf("resolved IP is: %s", ipAddr)
		ctx.Log.Info(msg, nil)
		dnsresource.IsReady = true
	}

	return &dnsresource, nil
}

func AssertResource(o ...any) bool {
	var result bool
	chk := func(i any) bool {
		v := reflect.ValueOf(i)
		if v.IsValid() && !v.IsZero() {
			return true
		}
		return false
	}

	if len(o) > 1 {
		for _, i := range o {
			result = chk(i)
			if !result {
				break
			}
		}
		return result
	}
	result = chk(o)
	return result
}

func NewWaitForUrl(ctx *pulumi.Context, url string, args *WaitForUrlArgs, opts ...pulumi.ResourceOption) (*WaitForUrl, error) {
	var urlresource WaitForUrl
	err := ctx.RegisterComponentResource("pkg:index:WaitForUrl", url, &urlresource, opts...)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	timeout := time.Duration(args.Timeout) * time.Minute
	retry := time.Duration(args.Retry) * time.Second

	req := func(url string) int {
		var r *http.Response
		var s int
		r, err = http.Get(url)
		if err != nil {
			func() {
				switch {
				case !strings.Contains(err.Error(), "no such host"):
					return
				case !strings.Contains(err.Error(), "i/o timeout"):
					return
				default:
					ctx.Log.Error(err.Error(), nil)
				}
			}()
			return s
		}
		defer r.Body.Close()
		return r.StatusCode
	}

	status := req(url)
	for time.Since(start) < timeout && status != args.Status {
		time.Sleep(retry)
		status = req(url)
	}
	if time.Since(start) >= timeout {
		msg := fmt.Sprintf("exceded timeout waiting for %s", url)
		ctx.Log.Error(msg, nil)
	}

	urlresource.Name = pulumi.String(url).ToStringOutput()
	urlresource.StatusCode = pulumi.Int(status).ToIntOutput()
	ctx.RegisterResourceOutputs(&urlresource, pulumi.Map{
		"url":        pulumi.ToOutput(pulumi.String(url)),
		"statusCode": pulumi.ToOutput(pulumi.Int(status)),
	})
	return &urlresource, nil
}
