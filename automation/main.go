package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
)

const (
	Blue    = "\033[34;1m"
	Green   = "\033[32;1m"
	Magenta = "\033[35;1m"
	Red     = "\033[31;1m"
	Reset   = "\033[0m"
)

type colorUp struct{}
type colorDestroy struct{}

func (colorUp) ApplyOption(opts *optup.Options) {
	opts.Color = "always"
}

func (colorDestroy) ApplyOption(opts *optdestroy.Options) {
	opts.Color = "always"
}

func main() {
	destroy := false
	args := os.Args[1:]
	if len(args) > 0 {
		if args[0] == "destroy" {
			destroy = true
		}
	}

	ctx := context.Background()
	stack := auto.FullyQualifiedStackName("bthompso", "apl-demo", "dev")
	workDir := filepath.Join("..", "apl")
	stdoutStreamer := optup.ProgressStreams(os.Stdout)

	s, err := auto.UpsertStackLocalSource(ctx, stack, workDir)
	if err != nil {
		fmt.Printf("%s[error]%sfailed to get local stack: %s\n", Red, Reset, stack)
	}
	fmt.Printf("%susing stack:%s %s\n", Magenta, Reset, stack)

	// destroy all resources
	if destroy {
		fmt.Printf("%sdestroying %s stack...%s\n", Green, stack, Reset)
		stdoutStreamer := optdestroy.ProgressStreams(os.Stdout)
		_, err = s.Destroy(ctx, stdoutStreamer, colorDestroy{})
		if err != nil {
			fmt.Printf("%s[error]%s failed to destroy stack: %v\n", Red, Reset, err)
			os.Exit(1)
		}
		fmt.Printf("\n%sstack destoyed%s\n", Green, Reset)
		os.Exit(0)
	}

	// provision cloud infra
	fmt.Printf("\n%s[info]%s (1/4)%s provisioning cloud resources\n\n", Green, Magenta, Reset)
	_, err = deployRun(ctx, s, stdoutStreamer, 0)
	if err != nil {
		fmt.Printf("%s[error]%s deploying cloud resources: %v\n", Red, Reset, err)
		os.Exit(1)
	}

	// configure dns
	fmt.Printf("\n%s[info]%s (2/4)%s create dns records\n\n", Green, Magenta, Reset)
	_, err = deployRun(ctx, s, stdoutStreamer, 1)
	if err != nil {
		fmt.Printf("%s[error]%s with dns configuration: %v\n", Red, Reset, err)
		os.Exit(1)
	}

	// wait for propagation
	fmt.Printf("\n%s[info]%s (3/4)%s wait for dns propagation\n\n", Green, Magenta, Reset)
	_, err = deployRun(ctx, s, stdoutStreamer, 2)
	if err != nil {
		fmt.Printf("%s[error]%s with dns propagation: %v\n", Red, Reset, err)
		os.Exit(1)
	}

	// deploy apl helm chart and get credentials
	fmt.Printf("\n%s[info]%s (4/4)%s deploying app platform\n\n", Green, Magenta, Reset)
	_, err = deployRun(ctx, s, stdoutStreamer, 3)
	if err != nil {
		fmt.Printf("%s[error] %sdeploying app platform: %v\n", Red, Reset, err)
		os.Exit(1)
	}
}

func deployRun(ctx context.Context, s auto.Stack, stdout optup.Option, n int) (*auto.UpResult, error) {
	stp, _ := s.GetConfig(ctx, "step")
	cpt, _ := s.GetConfig(ctx, "complete")
	stpInt, _ := strconv.Atoi(stp.Value)
	cptInt, _ := strconv.Atoi(cpt.Value)
	conf := map[string]int{
		"step":     stpInt,
		"complete": cptInt,
	}

	skip := func() {
		fmt.Printf("%s[info]%s skipping\n", Blue, Reset)
	}
	setConfig := func(conf map[string]int) {
		for k, v := range conf {
			value := strconv.Itoa(v)
			s.SetConfig(ctx, k, auto.ConfigValue{Value: value, Secret: false})
		}
	}
	resetConf := func() {
		conf["step"] = 0
		conf["complete"] = -1
		setConfig(conf)
	}

	run := func() (auto.UpResult, error) {
		res, err := s.Up(ctx, stdout, colorUp{})
		if err == nil {
			if n <= 3 {
				n++
			} else {
				n = 0
			}

			cptInt = n - 1
			conf["step"] = n
			conf["complete"] = cptInt
			setConfig(conf)
		}
		return res, err
	}

	switch {
	case stpInt == 0 || stpInt > 3:
		resetConf()
		stackRefresh(ctx, s)
	case stpInt == cptInt:
		conf["step"]++
		setConfig(conf)
		skip()
		return nil, nil
	case n < stpInt:
		if cptInt > stpInt {
			conf["complete"] = stpInt - 1
			setConfig(conf)
		}
		skip()
		return nil, nil
	}

	res, err := run()
	if err != nil {
		return nil, err
	}
	return &res, err
}

func chkStackErr(err error, s auto.Stack, a string) {
	if err != nil {
		st := fmt.Sprintf("%s: %v\n", s, err)
		switch a {
		case "destroy":
			fmt.Printf("%s[error]%s failed to destroy stack\n%s", Red, Reset, st)
		case "refresh":
			fmt.Printf("%s[error]%s faled to refresh stack\n%s", Red, Reset, st)
		case "upsert":
			fmt.Printf("%s[error]%s failed to get local stack\n%s", Red, Reset, st)
		}
		os.Exit(1)
	}
}

func stackRefresh(ctx context.Context, s auto.Stack) {
	_, err := s.Refresh(ctx)
	chkStackErr(err, s, "refresh")
	fmt.Printf("%s[info]%s refreshing state...\n\n", Green, Reset)
}
