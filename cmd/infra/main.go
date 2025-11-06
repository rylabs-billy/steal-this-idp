package main

import (
	"github.com/rylabs-billy/steal-this-idp/cmd/infra/app"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "linode")
		aplcfg := config.New(ctx, "apl")
		cfgData := map[string]string{
			"domain": aplcfg.Require("domain"),
			"label":  aplcfg.Require("label"),
			"email":  aplcfg.Require("email"),
			"region": aplcfg.Require("region"),
		}
		resources := make(map[string]interface{})

		infra := &app.PulumiResourceInfo{
			Data:      cfgData,
			Resources: resources,
			Token:     cfg.Require("token"),
		}
		err := infra.Build(ctx)
		if err != nil {
			return err
		}

		err = infra.Config(ctx)
		if err != nil {
			return err
		}

		return nil
	})
}
