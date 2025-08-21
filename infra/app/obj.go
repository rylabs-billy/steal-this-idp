package app

import (
	"fmt"

	"github.com/pulumi/pulumi-linode/sdk/v4/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type LinodeObjBucket struct {
	pulumi.ResourceState

	bucketName   pulumi.StringOutput `pulumi:"bucketName"`
	bucketRegion pulumi.StringOutput `pulumi:"bucketRegion"`
}

type LinodeObjBucketArgs struct {
	Region            string
	Key               *linode.ObjectStorageKey
	LifecyclePolicies linode.ObjectStorageBucketLifecycleRuleArray
}

func (obj *LinodeObjBucketArgs) SetDefaults() {
	// default region
	if obj.Region == "" {
		obj.Region = "us-ord"
	}
	// default lifecycle policy
	if len(obj.LifecyclePolicies) == 0 {
		obj.LifecyclePolicies = linode.ObjectStorageBucketLifecycleRuleArray{
			&linode.ObjectStorageBucketLifecycleRuleArgs{
				Id:                                 pulumi.String("global-expiration-policy"),
				Enabled:                            pulumi.Bool(true),
				AbortIncompleteMultipartUploadDays: pulumi.Int(5),
				Expiration: &linode.ObjectStorageBucketLifecycleRuleExpirationArgs{
					Days: pulumi.Int(90),
				},
			},
		}
	}
}

func NewLinodeObjBucket(ctx *pulumi.Context, bucketName string, bucketArgs *LinodeObjBucketArgs, opts ...pulumi.ResourceOption) (*LinodeObjBucket, error) {
	var objResource LinodeObjBucket
	args := bucketArgs
	args.SetDefaults()
	bucketResourceName := fmt.Sprintf("obj-%s", bucketName)

	err := ctx.RegisterComponentResource("pkg:index:LinodeObjBucket", bucketResourceName, &objResource, opts...)
	if err != nil {
		return nil, err
	}

	_, err = linode.NewObjectStorageBucket(ctx, bucketName, &linode.ObjectStorageBucketArgs{
		AccessKey:      args.Key.AccessKey,
		SecretKey:      args.Key.SecretKey,
		Region:         pulumi.String(args.Region),
		Label:          pulumi.String(bucketName),
		LifecycleRules: args.LifecyclePolicies,
	}, pulumi.Parent(&objResource))
	if err != nil {
		msg := fmt.Sprintf("error creating linode object storage bucket: %v", err)
		ctx.Log.Error(msg, nil)
	}

	objResource.bucketName = pulumi.String(bucketName).ToStringOutput()
	objResource.bucketRegion = pulumi.String(args.Region).ToStringOutput()
	ctx.RegisterResourceOutputs(&objResource, pulumi.Map{
		"bucketName":   pulumi.ToOutput(bucketName),
		"bucketRegion": pulumi.ToOutput(args.Region),
	})

	return &objResource, nil
}
