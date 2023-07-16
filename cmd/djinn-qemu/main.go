package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/andrewpillar/query"

	"djinn-ci.com/image"
	"djinn-ci.com/user"

	"djinn-ci.com/x/tools/internal"
)

func run(ctx context.Context, args []string) error {
	argv0 := args[0]

	var (
		handle    string
		namespace string
		timeout   string
		verbose   bool
	)

	fs := flag.NewFlagSet(argv0, flag.ExitOnError)
	fs.StringVar(&handle, "u", "", "the user the image belongs to, if any")
	fs.StringVar(&namespace, "n", "", "the namespace the image is in")
	fs.StringVar(&timeout, "t", "15", "the timeout for connecting to the machine")
	fs.BoolVar(&verbose, "v", false, "turn on verbose output")
	fs.Parse(args[1:])

	args = fs.Args()

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: %s [-n namespace] [-u user] [-t timeout] [-v] <image>\n", argv0)
		os.Exit(1)
		return nil
	}

	name := filepath.Join("_base", "qemu", "x86_64", args[0])

	if handle != "" {
		if err := internal.LoadEnv(); err != nil {
			return err
		}

		db, err := internal.DetectAndConnectDatabase(ctx)

		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
			os.Exit(1)
		}

		u, ok, err := user.NewStore(db).Get(ctx, user.WhereHandle(handle))

		if err != nil {
			return err
		}

		if !ok {
			return errors.New("no such user " + handle)
		}

		name = args[0]

		i, ok, err := image.NewStore(db).Get(
			ctx,
			query.Where("user_id", "=", query.Arg(u.ID)),
			func(q query.Query) query.Query {
				if namespace == "" {
					return q
				}
				return query.Where("namespace_id", "=", query.Select(
					query.Columns("id"),
					query.From("namespaces"),
					query.Where("path", "=", query.Arg(namespace)),
				))(q)
			},
			query.Where("name", "=", query.Arg(name)),
		)

		if err != nil {
			return err
		}

		if !ok {
			return err
		}
		name = filepath.Join(strconv.FormatInt(u.ID, 10), "qemu", i.Hash)
	}

	var cfg struct {
		Driver struct {
			QEMU struct {
				Disks  string
				Memory int64
				CPUs   int64
			}
		} `config:",nogroup"`
	}

	if err := internal.DecodeConfig(&cfg, filepath.Join(internal.ConfigDir, "driver.conf")); err != nil {
		return err
	}

	name = filepath.Join(cfg.Driver.QEMU.Disks, name)

	fmt.Println("Booting QEMU machine with image", args[0])

	proc, addr, port, err := RunQEMU(name, cfg.Driver.QEMU.Memory, cfg.Driver.QEMU.CPUs)

	if err != nil {
		return err
	}

	defer os.Remove(addr)
	defer proc.Kill()

	mon, err := NewMonitor("unix", addr, time.Second*10)

	if err != nil {
		return err
	}

	defer mon.Close()

	if err := mon.Connect(); err != nil {
		return err
	}

	outputFlag := "-q"

	if verbose {
		outputFlag = "-v"
	}

	ssharg := []string{
		outputFlag,
		"-o", "ConnectTimeout=" + timeout,
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "StrictHostKeyChecking=no",
		"-p", strconv.Itoa(port),
		"root@localhost",
	}

	fmt.Println("Connecting to machine via SSH")

	cmd := exec.Command("ssh", ssharg...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	code := cmd.ProcessState.ExitCode()

	if code == 0 {
		powerdown := Command{
			Execute: "system_powerdown",
		}

		if err := mon.Command(powerdown); err != nil {
			return err
		}

		fmt.Println("Powering down machine")

		for ev := range mon.Events() {
			if ev.Event == "SHUTDOWN" {
				break
			}
		}
		return nil
	}
	return nil
}

func main() {
	ctx := context.Background()

	if err := run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)
		os.Exit(1)
	}
	fmt.Println("Done")
}
