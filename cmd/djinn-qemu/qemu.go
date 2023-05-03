package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type response struct {
	Return any `json:",omitempty"`
	Error  struct {
		Class string
		Desc  string
	} `json:",omitempty"`
}

func (r *response) Err() error {
	if r.Error.Desc == "" {
		return nil
	}
	return errors.New(r.Error.Desc)
}

type streamResponse struct {
	buf []byte
	err error
}

type Event struct {
	Event string
	Data  map[string]any

	Timestamp struct {
		Seconds      int64
		Microseconds int64
	}
}

type Version struct {
	Package string
	QEMU    struct {
		Major int
		Micro int
		Minor int
	}
}

type Monitor struct {
	conn net.Conn

	listeners *int32

	stream chan streamResponse
	events chan Event

	Version      *Version
	Capabilities []string
}

func NewMonitor(network, addr string, timeout time.Duration) (*Monitor, error) {
	conn, err := net.DialTimeout(network, addr, timeout)

	if err != nil {
		return nil, err
	}

	return &Monitor{
		conn:      conn,
		listeners: new(int32),
		stream:    make(chan streamResponse),
		events:    make(chan Event),
	}, nil
}

type Command struct {
	Execute   string `json:"execute"`
	Arguments any    `json:"arguments,omitempty"`
}

func (m *Monitor) Command(cmd Command) error {
	if err := json.NewEncoder(m.conn).Encode(cmd); err != nil {
		return err
	}

	var resp response

	if err := json.NewDecoder(m.conn).Decode(&resp); err != nil {
		return err
	}

	if err := resp.Err(); err != nil {
		return err
	}
	return nil
}

func (m *Monitor) Events() chan Event {
	atomic.AddInt32(m.listeners, 1)
	return m.events
}

func (m *Monitor) Connect() error {
	var banner struct {
		QMP struct {
			Capabilities []string
			Version      Version
		}
	}

	if err := json.NewDecoder(m.conn).Decode(&banner); err != nil {
		return err
	}

	m.Version = &banner.QMP.Version
	m.Capabilities = banner.QMP.Capabilities

	if err := m.Command(Command{Execute: "qmp_capabilities"}); err != nil {
		return err
	}

	go func() {
		defer close(m.stream)
		defer close(m.events)

		sc := bufio.NewScanner(m.conn)

		for sc.Scan() {
			var ev Event

			buf := sc.Bytes()

			if err := json.Unmarshal(buf, &ev); err != nil {
				continue
			}

			if ev.Event == "" {
				m.stream <- streamResponse{buf: buf}
				continue
			}

			if atomic.LoadInt32(m.listeners) == 0 {
				continue
			}
			m.events <- ev
		}

		if err := sc.Err(); err != nil {
			m.stream <- streamResponse{err: err}
		}
	}()

	return nil
}

func (m *Monitor) Close() error {
	err := m.conn.Close()

	for range m.stream {}

	return err
}

func Snapshot(disk string) (string, error) {
	f, err := os.CreateTemp("", "djinn-qemu-snapshot-*.img")

	if err != nil {
		return "", err
	}

	f.Close()

	arg := []string{
		"create",
		"-f", "qcow2",
		"-b", disk,
		"-F", "qcow2",
		f.Name(),
	}

	cmd := exec.Command("qemu-img", arg...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func Commit(disk string) error {
	cmd := exec.Command("qemu-img", "commit", disk)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func RunQEMU(disk string, mem, cpus int64) (*os.Process, string, int, error) {
	const tcpMaxPort = 65535

	port := 2222

	var buf bytes.Buffer

	for port < tcpMaxPort {
		pidfile, err := os.CreateTemp("", "djinn-qemu-*.pid")

		if err != nil {
			return nil, "", 0, err
		}

		pidfile.Close()

		sockfile, err := os.CreateTemp("", "djinn-qemu-monitor-*.sock")

		if err != nil {
			return nil, "", 0, err
		}

		sockfile.Close()

		hostfwd := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

		arg := []string{
			"-qmp", "unix:" + sockfile.Name() + ",server,nowait",
			"-enable-kvm",
			"-daemonize",
			"-display", "none",
			"-pidfile", pidfile.Name(),
			"-smp", strconv.FormatInt(cpus, 10),
			"-m", strconv.FormatInt(mem, 10),
			"-net", "nic,model=virtio",
			"-net", "user,hostfwd=tcp:" + hostfwd + "-:22",
			"-drive", "file=" + disk + ",media=disk,if=virtio",
		}

		cmd := exec.Command("qemu-system-x86_64", arg...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = &buf

		if err := cmd.Run(); err != nil {
			os.Remove(pidfile.Name())
			os.Remove(sockfile.Name())

			if strings.Contains(buf.String(), "Could not set up host forwarding rule") {
				port++
				buf.Reset()
				continue
			}
			return nil, "", 0, errors.New(buf.String())
		}

		b, err := os.ReadFile(pidfile.Name())

		if err != nil {
			return nil, "", 0, err
		}

		pid, err := strconv.Atoi(string(b[:len(b)-1]))

		if err != nil {
			return nil, "", 0, err
		}

		proc, err := os.FindProcess(pid)

		if err != nil {
			return nil, "", 0, err
		}
		return proc, sockfile.Name(), port, nil
	}
	return nil, "", 0, errors.New("exhausted ports for host forwarding")
}
