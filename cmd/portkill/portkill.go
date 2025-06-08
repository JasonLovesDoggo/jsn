package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"pkg.jsn.cam/jsn/flagenv"
)

var (
	force   = flag.Bool("f", false, "Force kill the process (SIGKILL instead of SIGTERM)")
	list    = flag.Bool("l", false, "List processes using the port but don't kill them")
	verbose = flag.Bool("v", false, "Verbose output")
)

func main() {
	// Configure flagenv and program info
	flagenv.ParseWithPrefix("PORTKILL_")

	if flag.NArg() == 0 {
		printUsage()
		os.Exit(1)
	}

	for _, arg := range flag.Args() {
		port, err := strconv.Atoi(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %q is not a valid port number\n", arg)
			os.Exit(1)
		}

		if err := handlePort(port); err != nil {
			fmt.Fprintf(os.Stderr, "Error handling port %d: %v\n", port, err)
			os.Exit(1)
		}
	}
}

func handlePort(port int) error {
	pids, err := findPIDsByPort(port)
	if err != nil {
		return fmt.Errorf("failed to find PIDs using port %d: %w", port, err)
	}

	if len(pids) == 0 {
		fmt.Printf("No processes found using port %d\n", port)
		return nil
	}

	for _, pid := range pids {
		procInfo, err := getProcessInfo(pid)
		if err != nil {
			return fmt.Errorf("failed to get process info for PID %s: %w", pid, err)
		}

		if *list {
			fmt.Printf("PID %s: %s\n", pid, procInfo)
		} else {
			signal := "TERM"
			if *force {
				signal = "KILL"
			}

			if *verbose {
				fmt.Printf("Killing process %s (%s) with SIG%s\n", pid, procInfo, signal)
			} else {
				fmt.Printf("Killing process %s with SIG%s\n", pid, signal)
			}

			cmd := exec.Command("kill", fmt.Sprintf("-%s", signal), pid)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to kill process %s: %w", pid, err)
			}
		}
	}

	return nil
}

func findPIDsByPort(port int) ([]string, error) {
	// Use lsof to find processes using the port
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-t")
	output, err := cmd.Output()
	if err != nil {
		// lsof returns error if no processes found, which isn't an error for us
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []string{}, nil
		}
		return nil, err
	}

	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string
	for _, pid := range pids {
		if pid != "" {
			result = append(result, pid)
		}
	}

	return result, nil
}

func getProcessInfo(pid string) (string, error) {
	cmd := exec.Command("ps", "-p", pid, "-o", "comm=")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: portkill [options] port [port...]\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
}
