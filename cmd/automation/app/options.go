package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	flag "github.com/spf13/pflag"
)

type clicmd struct {
	action     bool
	name       string
	subcommand string
	stacks     stackMap
}

func (c *clicmd) Doit(ctx context.Context) {
	doit(ctx, c)
}

func Init(ctx context.Context, stks stackMap, args []string) *clicmd {
	cmd := &clicmd{
		subcommand: "all",
		stacks:     stks,
	}

	if len(args) == 1 {
		cmd.name = args[0]
		println(Usage(cmd))
		os.Exit(0)
	}

	if len(args) > 1 {
		switch args[1] {
		case "create":
			cmd.action = true
		case "destroy":
			cmd.action = false
		default:
			println(Usage(cmd))
			os.Exit(0)
		}
	}

	cmd.name = args[1]
	fs := flag.NewFlagSet(cmd.name, flag.ExitOnError)
	fs.Usage = func() {
		usage := Usage(cmd)
		fmt.Println(usage)
		os.Exit(0)
	}

	for k := range stks {
		short := k[:1]
		fs.BoolP(k, short, false, Usage(cmd))
		fs.Lookup(k).NoOptDefVal = "true"
	}

	_ = fs.Parse(args[1:])

	if fs.Parsed() {
		for k := range stks {
			f := fs.Lookup(k)
			if f.Changed {
				cmd.subcommand = k
			}
		}

	}

	return cmd
}

func doit(ctx context.Context, cmd *clicmd) {
	up := func(stk microStack) {
		stdout := optup.ProgressStreams(os.Stdout)
		s := initLocalStack(ctx, stk)
		s.Refresh(ctx)
		msg("deploying", stk.fqsn, nil)
		res, err := s.Up(ctx, stdout, colorUp{})
		msg("deploy", stk.fqsn, err)

		if stk.name == "infra" {
			if nbid, ok := res.Outputs["loadbalancerId"]; ok {
				s.SetConfig(ctx, "nodebalancer-id", auto.ConfigValue{Value: nbid.Value.(string)})
			}
		}

	}

	down := func(stk microStack) {
		stdout := optdestroy.ProgressStreams(os.Stdout)
		s := initLocalStack(ctx, stk)
		msg("destroying", stk.fqsn, nil)
		s.Refresh(ctx)
		_, err := s.Destroy(ctx, stdout, colorDestroy{}, parallelism{})
		msg("destroy", stk.fqsn, err)

		if stk.name == "infra" {
			s.RemoveConfig(ctx, "nodebalancer-id")
		}
	}

	st, ok := cmd.stacks[cmd.subcommand]
	action := cmd.action
	switch action {
	case true:
		if !ok {
			// infra
			stk := cmd.stacks["infra"]
			up(stk)

			// apl
			stk = cmd.stacks["apl"]
			up(stk)
		} else {
			up(st)
		}
	case false: // tear down in reverse order
		if !ok {
			// apl
			stk := cmd.stacks["apl"]
			down(stk)

			// infra
			stk = cmd.stacks["infra"]
			down(stk)
		} else {
			down(st)
		}
	}
	os.Exit(0)
}

func Usage(c *clicmd) string {
	var (
		b           strings.Builder
		cols        = "\n%-2s %-12s%s  %-10s  %s  %s\n"
		usageCols   = "\n%-2s %-12s%s  %s  %s  %s\n"
		createDesc  = "deploy a stack by name, or run without options to deploy them all"
		destroyDesc = "provide a stack name to destory, or leave blank to destroy everything"
		defaultDesc = "run without options to target all stacks, or provide a specific stack name"
		msg         = fmt.Sprintf(usageCols, Magenta, "usage:", Grey, prog, "[ARG]  [OPTION]", Reset)
	)

	switch c.name {
	case "create", "destroy":
		var desc string
		if c.name == "create" {
			desc = createDesc
		} else {
			desc = destroyDesc
		}
		m := fmt.Sprintf("%s  [OPTION]", c.name)
		msg = fmt.Sprintf(usageCols, Magenta, "usage:", Grey, prog, m, Reset)
		msg += fmt.Sprintf(cols, Magenta, "description:", Grey, desc, "", Reset)
	default:
		msg += fmt.Sprintf(cols, Magenta, "description:", Grey, defaultDesc, "", Reset)
		msg += fmt.Sprintf(cols, Magenta, "arguments:", Grey, "", "", Reset)
		cd := fmt.Sprintf(cols, Magenta, "", Grey, "create", createDesc, Reset)
		dd := fmt.Sprintf(cols, Magenta, "", Grey, "destroy", destroyDesc, Reset)
		msg += strings.TrimLeft(cd, "\n")
		msg += strings.TrimLeft(dd, "\n")
	}

	msg += fmt.Sprintf(cols, Magenta, "options:", Grey, "", "", Reset)
	cd := fmt.Sprintf(cols, Magenta, "", Grey, "-a,  --apl", "", Reset)
	dd := fmt.Sprintf(cols, Magenta, "", Grey, "-i,  --infra", "", Reset)
	msg += strings.TrimLeft(cd, "\n")
	msg += strings.TrimLeft(dd, "\n")
	b.WriteString(msg)

	return fmt.Sprintf("%v", b.String())
}
