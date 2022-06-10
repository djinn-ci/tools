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
		Database struct {
			Addr string
			Name string

			Username string
			Password string
		}
	}

	if err := internal.DecodeConfig(&cfg, name); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}

	host, port, err := net.SplitHostPort(cfg.Database.Addr)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}

	os.Setenv("PGPASSWORD", cfg.Database.Password)

	flags := []string{
		"-h", host,
		"-p", port,
		"-U", cfg.Database.Username,
		"-d", cfg.Database.Name,
	}

	cmd := exec.Command("psql", flags...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}
}
