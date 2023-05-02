package main

import (
	"context"
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

func main() {
	argv0 := os.Args[0]

	var (
		handle  string
		timeout string
	)

	fs := flag.NewFlagSet(argv0, flag.ExitOnError)
	fs.StringVar(&handle, "u", "", "the user the image belongs to, if any")
	fs.StringVar(&timeout, "t", "15", "the timeout for connecting to the machine")
	fs.Parse(os.Args[1:])

	args := fs.Args()

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: %s [-t timeout] [-u user] <image>\n", argv0)
		os.Exit(1)
	}

	name := filepath.Join("_base", "qemu", "x86_64", args[0])

	if handle != "" {
		ctx := context.Background()

		db, err := internal.DetectAndConnectDatabase(ctx)

		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
			os.Exit(1)
		}

		u, ok, err := user.NewStore(db).Get(ctx, user.WhereHandle(handle))

		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
			os.Exit(1)
		}

		if !ok {
			fmt.Fprintf(os.Stderr, "%s: no such user %s\n", argv0, handle)
			os.Exit(1)
		}

		_, ok, err = image.NewStore(db).Get(
			ctx,
			query.Where("user_id", "=", query.Arg(u.ID)),
			query.Where("name", "=", query.Arg(name)),
		)

		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
			os.Exit(1)
		}

		if !ok {
			fmt.Fprintf(os.Stderr, "%s: no such image %s\n", argv0, name)
			os.Exit(1)
		}
		name = filepath.Join(strconv.FormatInt(u.ID, 10), "qemu", name)
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
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}

	name, err := Snapshot(filepath.Join(cfg.Driver.QEMU.Disks, name))

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}

	defer os.Remove(name)

	fmt.Println("Created snapshot of image", args[0])
	fmt.Println("Booting QEMU machine with image", args[0])

	proc, addr, port, err := RunQEMU(name, cfg.Driver.QEMU.Memory, cfg.Driver.QEMU.CPUs)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to start qemu\n", argv0)
		os.Exit(1)
	}

	defer os.Remove(addr)
	defer proc.Kill()

	mon, err := NewMonitor("unix", addr, time.Second * 10)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}

	defer mon.Close()

	if err := mon.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}

	ssharg := []string{
		"-q",
		"-o", "ConnectTimeout=" + timeout,
		"-o", "UserKnownHostsFile=/dev/null",
		"-p", strconv.Itoa(port),
		"root@localhost",
	}

	fmt.Println("Connecting to machine via SSH")

	cmd := exec.Command("ssh", ssharg...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}

	powerdown := Command{
		Execute: "system_powerdown",
	}

	if err := mon.Command(powerdown); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}

	for ev := range mon.Events() {
		if ev.Event == "SHUTDOWN" {
			break
		}
	}

	time.Sleep(time.Second)

	if err := Commit(name); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}
}
