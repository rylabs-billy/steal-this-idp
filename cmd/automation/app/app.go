package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	utils "github.com/rylabs-billy/steal-this-idp/utils"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type colorUp struct{}

type colorDestroy struct{}

type parallelism struct{}

type microStack struct {
	fqsn     string
	name     string
	buildFn  pulumi.RunFunc
	configFn pulumi.RunFunc
}

type stackMap map[string]microStack

func (colorUp) ApplyOption(opts *optup.Options) {
	opts.Color = "always"
}

func (colorDestroy) ApplyOption(opts *optdestroy.Options) {
	opts.Color = "always"
}

func (parallelism) ApplyOption(opts *optdestroy.Options) {
	opts.Parallel = 4
}

const (
	prog    = "aplcli"
	stack   = "dev"
	project = "apl-demo"
	org     = "bthompso"
	Blue    = "\033[34;1m"
	Green   = "\033[32;1m"
	Grey    = "\033[37;1m"
	Magenta = "\033[35;1m"
	Red     = "\033[31;1m"
	Reset   = "\033[0m"
)

func Run() {
	ctx := context.Background()

	infraStack := microStack{
		fqsn:    fmt.Sprintf("%s/%s-infra/%s", org, project, stack),
		name:    "infra",
		buildFn: nil,
	}
	aplStack := microStack{
		fqsn: fmt.Sprintf("%s/%s/%s", org, project, stack),
		name: "apl",
	}

	stkMap := stackMap{
		"infra": infraStack,
		"apl":   aplStack,
	}

	cmd := Init(ctx, stkMap, os.Args)
	cmd.Doit(ctx)
}

func initLocalStack(ctx context.Context, stk microStack) auto.Stack {
	dir := stk.name
	fqsn := stk.fqsn

	ws := filepath.Join("..", dir, "app")
	s, err := auto.UpsertStackLocalSource(ctx, fqsn, ws)
	if err != nil {
		fmt.Printf("\n%s%-10s %s failed to get local stack: %s %s\n", Red, "[error]", Grey, fqsn, Reset)
		fmt.Printf("%s%-10s %v %s\n", Grey, "", err, Reset)
		os.Exit(1)
	}

	if utils.AssertResource(stk.buildFn) {
		s.Workspace().SetProgram(stk.buildFn)
	}

	if utils.AssertResource(stk.configFn) {
		s.Workspace().SetProgram(stk.configFn)
	}

	fmt.Printf("\n%s%-10s %s using stack: %s %s\n", Green, "[info]", Grey, fqsn, Reset)
	return s
}

func msg(opt, stk string, err error) {
	var (
		cols1 = "\n%s%-10s %s stack %s: %s %s\n"
		cols2 = "\n%s%-10s %s failed to %s stack: %s%s\n"
		cols3 = "%s%-10s %s %v%s\n"
		cols4 = "\n%s%-10s %s %s stack: %s %s\n"
	)
	switch opt {
	case "deploy", "destroy":
		if err == nil {
			fmt.Printf(cols1, Green, "[info]", Grey, opt, stk, Reset)
		} else {
			fmt.Printf(cols2, Red, "[error]", Grey, opt, stk, Reset)
			fmt.Printf(cols3, Red, "[error]", Grey, err, Reset)
			os.Exit(1)
		}
	case "deploying", "destroying":
		fmt.Printf(cols4, Green, "[info]", Grey, opt, stk, Reset)
	case "invalid":
		fmt.Printf(cols3, Red, "[error]", Grey, err, Reset)
		os.Exit(1)
	}
}
