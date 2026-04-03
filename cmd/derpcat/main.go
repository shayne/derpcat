package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
)

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func runMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	stopCPUProfile, err := startCPUProfileFromEnv()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer stopCPUProfile()

	stopBlockProfile, err := startBlockProfileFromEnv()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer stopBlockProfile()

	return run(args, stdin, stdout, stderr)
}

func startCPUProfileFromEnv() (func(), error) {
	path := os.Getenv("DERPCAT_CPU_PROFILE")
	if path == "" {
		return func() {}, nil
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		_ = file.Close()
		return nil, err
	}

	return func() {
		pprof.StopCPUProfile()
		_ = file.Close()
	}, nil
}

func startBlockProfileFromEnv() (func(), error) {
	path := os.Getenv("DERPCAT_BLOCK_PROFILE")
	if path == "" {
		return func() {}, nil
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	runtime.SetBlockProfileRate(1)

	return func() {
		defer file.Close()
		defer runtime.SetBlockProfileRate(0)
		_ = pprof.Lookup("block").WriteTo(file, 0)
	}, nil
}
