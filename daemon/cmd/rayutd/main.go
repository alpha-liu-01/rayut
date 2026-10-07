package main

import (
	"errors"
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
	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
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
	if err := paths.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if os.Getenv("RAYUTD_CHILD") != "1" {
		supervise()
	}
	if err := cgroup.LeaveAppScope(os.Getpid()); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	_ = os.Remove(paths.Ready)
	if err := ensureSingle(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ln, err := net.Listen("tcp", api.ListenAddr)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			fmt.Fprintln(os.Stderr, "already-running")
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	tcp, ok := ln.(*net.TCPListener)
	if !ok || !tcp.Addr().(*net.TCPAddr).IP.IsLoopback() {
		ln.Close()
		fmt.Fprintln(os.Stderr, "refusing non-loopback listener")
		os.Exit(1)
	}
	if err := route.Recover(); err != nil {
		ln.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	server, err := api.New()
	if err != nil {
		ln.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(paths.Ready, []byte("ok\n"), 0o600); err != nil {
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
	_ = os.Remove(paths.HelperPid)
	_ = os.Remove(paths.Ready)
}

func supervise() {
	_ = os.Remove(paths.Ready)
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
		if _, err := os.Stat(paths.Ready); err == nil {
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
	path := paths.HelperPid
	data, err := os.ReadFile(path)
	if err == nil {
		pid, conv := strconv.Atoi(strings.TrimSpace(string(data)))
		if conv == nil && pid > 0 && syscall.Kill(pid, 0) == nil {
			return fmt.Errorf("already-running")
		}
	}
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600)
}
