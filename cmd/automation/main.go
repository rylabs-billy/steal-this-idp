package main

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

const (
	Blue    = "\033[34;1m"
	Green   = "\033[32;1m"
	Grey    = "\033[37;1m"
	Magenta = "\033[35;1m"
	Red     = "\033[31;1m"
	Reset   = "\033[0m"
)

type colorUp struct{}

type colorDestroy struct{}

type parallelism struct{}

type microStack struct {
	fqsn, name        string
	buildFn, configFn pulumi.RunFunc
	stdout            interface{}
}

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
	stack   = "dev"
	project = "apl-demo"
	org     = "bthompso"
)

func main() {
	var (
		destroy    bool
		infra, apl auto.Stack
		ctx        = context.Background()
	)

	// configure local stack info for workspaces
	infraStack := microStack{
		fqsn:    fmt.Sprintf("%s/%s-infra/%s", org, project, stack),
		name:    "infra",
		buildFn: nil,
	}
	aplStack := microStack{
		fqsn: fmt.Sprintf("%s/%s/%s", org, project, stack),
		name: "apl",
	}

	// command line args
	args := os.Args[1:]
	if len(args) > 0 {
		if args[0] == "destroy" {
			destroy = true
		}
	}

	// destroy
	if destroy {
		stdout := optdestroy.ProgressStreams(os.Stdout)

		// target a specific stack to destroy
		if len(args) == 2 {
			var s auto.Stack
			var fqsn, n string

			switch args[1] {
			case "apl":
				s = initLocalStack(ctx, aplStack)
				fqsn = aplStack.fqsn
				n = aplStack.name
			case "infra":
				s = initLocalStack(ctx, infraStack)
				fqsn = infraStack.fqsn
				n = infraStack.name
			default:
				err := fmt.Errorf("invalid arguments: %s", args[1])
				msg("invalid", "", err)
			}

			msg("destroying", fqsn, nil)
			s.Refresh(ctx)
			_, err := s.Destroy(ctx, stdout, colorDestroy{})
			msg("destroy", fqsn, err)
			if n == "infra" {
				s.RemoveConfig(ctx, "nodebalancer-id")
			}
			os.Exit(0)
		}

		// destroy everything
		if len(args) == 1 {
			// aplstack
			apl = initLocalStack(ctx, aplStack)
			msg("destroying", aplStack.fqsn, nil)

			apl.Refresh(ctx)
			_, err := apl.Destroy(ctx, stdout, colorDestroy{}, parallelism{})
			msg("destroy", aplStack.fqsn, err)

			// infra
			infra = initLocalStack(ctx, infraStack)
			msg("destroying", infraStack.fqsn, nil)

			infra.Refresh(ctx)
			_, err = infra.Destroy(ctx, stdout, colorDestroy{}, parallelism{})
			msg("destroy", infraStack.fqsn, err)
			infra.RemoveConfig(ctx, "nodebalancer-id")
			os.Exit(0)
		}
	}

	// target a specific stack for deployment
	stdout := optup.ProgressStreams(os.Stdout)
	if len(args) == 1 {
		var (
			s    auto.Stack
			fqsn string
		)
		switch args[0] {
		case "apl":
			s = initLocalStack(ctx, aplStack)
			fqsn = aplStack.fqsn
		case "infra":
			s = initLocalStack(ctx, infraStack)
			fqsn = infraStack.fqsn
		default:
			err := fmt.Errorf("invalid arguments: %s", args[0])
			msg("invalid", "", err)
		}

		msg("deploying", fqsn, nil)
		s.Refresh(ctx)

		res, err := s.Up(ctx, stdout, colorUp{})
		msg("deploy", fqsn, err)
		// log nodebalancer id in stack config meta
		if nbid, ok := res.Outputs["loadbalancerId"]; ok {
			s.SetConfig(ctx, "nodebalancer-id", auto.ConfigValue{Value: nbid.Value.(string)})
		}
		os.Exit(0)
	}

	// deploy everything
	// infra
	infra = initLocalStack(ctx, infraStack)
	infra.Refresh(ctx)

	msg("deploying", infraStack.fqsn, nil)
	res, err := infra.Up(ctx, stdout, colorUp{})
	msg("deploy", infraStack.fqsn, err)

	if nbid, ok := res.Outputs["loadbalancerId"]; ok {
		infra.SetConfig(ctx, "nodebalancer-id", auto.ConfigValue{Value: nbid.Value.(string)})
	}

	// apl
	apl = initLocalStack(ctx, aplStack)
	apl.Refresh(ctx)

	msg("deploying", aplStack.fqsn, nil)
	_, err = apl.Up(ctx, stdout, colorUp{})
	msg("deploy", aplStack.fqsn, err)
	os.Exit(0)
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
	switch opt {
	case "deploy", "destroy":
		if err == nil {
			fmt.Printf("\n%s%-10s %s stack %s: %s %s\n", Green, "[info]", Grey, opt, stk, Reset)
		} else {
			fmt.Printf("\n%s%-10s %s failed to %s stack: %s%s\n", Red, "[error]", Grey, opt, stk, Reset)
			fmt.Printf("%s%-10s %s %v%s\n", Red, "[error]", Grey, err, Reset)
			os.Exit(1)
		}
	case "deploying", "destroying":
		fmt.Printf("\n%s%-10s %s %s stack: %s %s\n", Green, "[info]", Grey, opt, stk, Reset)
	case "invalid":
		fmt.Printf("%s%-10s %s %v%s\n", Red, "[error]", Grey, err, Reset)
		os.Exit(1)
	}

}

// func createDestroy(ctx context.Context, key string, stmap stackMap) (interface{}, error) {
// 	// create or destory micro stacks
// 	stk := stmap[key]
// 	switch v := stk.stdout.(type) {
// 	case optup.Option:
// 		s := stk.Load(ctx)
// 		res, err := s.Up(ctx, v, colorUp{})
// 		// log nodebalancer id in stack config meta
// 		if nbid, ok := res.Outputs["loadbalancerId"]; ok {
// 			s.SetConfig(ctx, "nodebalancer-id", auto.ConfigValue{Value: nbid.Value.(string)})
// 		}
// 		return res, err
// 	case optdestroy.Option:
// 		s := stk.Load(ctx)
// 		res, err := s.Destroy(ctx, v, colorDestroy{})
// 		if err == nil && stk.name == "infra" {
// 			s.RemoveConfig(ctx, "nodebalancer-id")
// 		}
// 		return res, err
// 	default:
// 		return nil, fmt.Errorf("result interface is nil")
// 	}
// }
