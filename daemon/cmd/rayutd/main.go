package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/alpha-liu-01/rayut/daemon/internal/api"
	"github.com/alpha-liu-01/rayut/daemon/internal/cgroup"
	"github.com/alpha-liu-01/rayut/daemon/internal/core"
	"github.com/alpha-liu-01/rayut/daemon/internal/route"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "--session" {
		fmt.Fprintln(os.Stderr, "usage: rayutd --session")
		os.Exit(1)
	}
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "rayutd --session must run as root")
		os.Exit(1)
	}
	if os.Getenv("RAYUTD_CHILD") != "1" {
		supervise()
	}
	if err := cgroup.LeaveAppScope(os.Getpid()); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if err := os.MkdirAll(core.Runtime, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = os.Remove(core.Runtime + "/ready")
	if err := ensureSingle(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := route.Recover(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	server, err := api.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ln, err := net.Listen("tcp", api.ListenAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	tcp, ok := ln.(*net.TCPListener)
	if !ok || !tcp.Addr().(*net.TCPAddr).IP.IsLoopback() {
		ln.Close()
		fmt.Fprintln(os.Stderr, "refusing non-loopback listener")
		os.Exit(1)
	}
	if err := os.WriteFile(core.Runtime+"/ready", []byte("ok\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(ln) }()
	fmt.Fprintln(os.Stderr, "rayutd listening")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
	case err := <-errCh:
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	if err := server.StopTun(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	_ = server.Shutdown()
	_ = os.Remove(core.Runtime + "/rayutd.pid")
	_ = os.Remove(core.Runtime + "/ready")
}

func supervise() {
	_ = os.Remove(core.Runtime + "/ready")
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append(os.Environ(), "RAYUTD_CHILD=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(core.Runtime + "/ready"); err == nil {
			os.Exit(0)
		}
		var status syscall.WaitStatus
		wpid, err := syscall.Wait4(cmd.Process.Pid, &status, syscall.WNOHANG, nil)
		if err == nil && wpid == cmd.Process.Pid {
			if status.Exited() {
				os.Exit(status.ExitStatus())
			}
			os.Exit(1)
		}
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "helper did not become ready")
	os.Exit(1)
}

func ensureSingle() error {
	path := core.Runtime + "/rayutd.pid"
	data, err := os.ReadFile(path)
	if err == nil {
		pid, conv := strconv.Atoi(strings.TrimSpace(string(data)))
		if conv == nil && pid > 0 && syscall.Kill(pid, 0) == nil {
			return fmt.Errorf("already-running")
		}
	}
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600)
}
