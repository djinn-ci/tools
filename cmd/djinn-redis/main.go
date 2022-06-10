package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"

	"djinn-ci.com/x/tools/internal"
)

func main() {
	argv0 := os.Args[0]

	if err := internal.LoadEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}

	name, err := internal.DetectConfig()

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}

	var cfg struct {
		Redis struct {
			Addr     string
			Password string
		}
	}

	if err := internal.DecodeConfig(&cfg, name); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}

	host, port, err := net.SplitHostPort(cfg.Redis.Addr)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}

	flags := []string{
		"-h", host,
		"-p", port,
		"-a", cfg.Redis.Password,
	}

	cmd := exec.Command("redis-cli", flags...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}
}
