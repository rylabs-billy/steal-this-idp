package app

import (
	"github.com/pulumi/pulumi-linode/sdk/v4/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func defaultLifecyclePolicy() linode.ObjectStorageBucketLifecycleRuleArray {
	lifecyclePolicy := linode.ObjectStorageBucketLifecycleRuleArray{
		&linode.ObjectStorageBucketLifecycleRuleArgs{
			Id:                                 pulumi.String("global-expiration-policy"),
			Enabled:                            pulumi.Bool(true),
			AbortIncompleteMultipartUploadDays: pulumi.Int(5),
			Expiration: &linode.ObjectStorageBucketLifecycleRuleExpirationArgs{
				Days: pulumi.Int(90),
			},
		},
	}

	return lifecyclePolicy
}
