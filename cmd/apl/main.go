package main

import (
	"github.com/rylabs-billy/steal-this-idp/cmd/apl/app"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "linode")
		aplcfg := config.New(ctx, "apl")
		aplVars := map[string]string{
			"domain":      aplcfg.Require("domain"),
			"infraSlug":   aplcfg.Require("infraSlug"),
			"region":      aplcfg.Require("region"),
			"email":       aplcfg.Require("email"),
			"label":       aplcfg.Require("label"),
			"otomiAdmin":  aplcfg.Require("otomiAdminPassword"),
			"teamDevelop": aplcfg.Require("teamDevelopPassword"),
			"ageKey":      aplcfg.Require("agePublicKey"),
			"agePrivKey":  aplcfg.Require("agePrivateKey"),
			"lokiAdmin":   aplcfg.Require("lokiAdminPassword"),
		}
		apl := app.AplResourceInfo{
			Token: cfg.Require("token"),
			Apl:   aplVars,
		}
		err := apl.Run(ctx)
		if err != nil {
			return err
		}
		return nil
	})
}
