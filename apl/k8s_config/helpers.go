package k8s_config

import (
	"fmt"
	"reflect"

	utils "github.com/rylabs-billy/apl-demo/apl/internal"

	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	v1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi-linode/sdk/v4/go/linode"

	// "github.com/pulumi/pulumi/sdk/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"gopkg.in/yaml.v2"
)

func RegisterSecret(ctx *pulumi.Context, secretData KubeSecrets, depends ...pulumi.Resource) *corev1.Secret {
	sec, err := corev1.NewSecret(ctx, secretData.Name, &corev1.SecretArgs{
		Data: pulumi.ToStringMap(secretData.Data),
		Kind: pulumi.String(secretData.Kind),
		Metadata: &v1.ObjectMetaArgs{
			Name:      pulumi.String(secretData.Name),
			Namespace: pulumi.String(secretData.Namespace),
		},
	}, pulumi.Provider(secretData.Provider), pulumi.DependsOn(depends))
	if err != nil {
		msg := fmt.Sprintf("error registering kubernetes secret: %v", err)
		ctx.Log.Error(msg, nil)
	}
	// return secret object
	return sec
}

func ExportSecret(ctx *pulumi.Context, sec *corev1.Secret, key string) pulumi.StringOutput {
	secret := sec.Data.ApplyT(func(data map[string]string) string {
		enc := data[key]
		dec := utils.DecodeString(enc)
		v := reflect.ValueOf(dec)
		if v.IsValid() && !v.IsZero() {
			return dec
		} else {
			msg := fmt.Sprintf("error exporting kubernetes secret: %s\n", data[key])
			msg += fmt.Sprintf("value returned was: '%v'", v)
			ctx.Log.Error(msg, nil)
		}
		return ""
	}).(pulumi.StringOutput)
	return secret
}

func CreateNamespace(ctx *pulumi.Context, args KubeNamespace, depends ...pulumi.Resource) *corev1.Namespace {
	namespace, err := corev1.NewNamespace(ctx, args.Name, &corev1.NamespaceArgs{
		Metadata: &v1.ObjectMetaArgs{
			Name: pulumi.String(args.Namespace),
		},
	}, pulumi.Provider(args.Provider), pulumi.DependsOn(depends))
	if err != nil {
		msg := fmt.Sprintf("error creating %s namespace: %v", args.Namespace, err)
		fmt.Println(msg)
		ctx.Log.Error(msg, nil)
	}
	return namespace
}

func LkeContext(ctx *pulumi.Context, cluster *linode.LkeCluster, kubecfg *KubeConfig) {
	cluster.Kubeconfig.ApplyT(func(k string) error {
		dec := utils.DecodeString(k)
		err := yaml.Unmarshal([]byte(dec), &kubecfg)
		if err != nil {
			// e := fmt.Errorf("%v", err)
			msg := fmt.Sprintf("an error occured while unmarshalling yaml: %v", fmt.Errorf("%v", err))
			return ctx.Log.Error(msg, nil)
		}
		return nil
	})
}
