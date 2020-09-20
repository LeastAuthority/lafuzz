package main

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/leastauthority/lafuzz/cmd/config"
)

var (
	cmdFuzz = &cobra.Command{
		Use:   "fuzz <pkg> <fuzz function>",
		Short: "Run go-fuzz against a fuzz function",
		Args:  cobra.ExactArgs(2),
		RunE:  runFuzz,
	}

	buildBin bool
	procs    int
)

func init() {
	cmdFuzz.Flags().BoolVarP(&buildBin, "build", "b", true, "if true, rebuilds test binary before running (default: true)")
	cmdFuzz.Flags().IntVarP(&procs, "procs", "p", 1, "number of processors to use (passed to go-fuzz's -procs flag)")
}

func absRepoRoot() string {
	absoluteOutputRoot, err := filepath.Abs(viper.GetString(config.RepoRoot))
	if err != nil {
		panic(err)
	}
	return absoluteOutputRoot
}
func runFuzz(cmd *cobra.Command, args []string) error {
	pkgPath := args[0]
	fuzzFuncName := args[1]
	repoRoot := absRepoRoot() //viper.GetString(config.RepoRoot)
	var build string
	if buildBin {
		build = "-b"
	}

	// TODO: factor out
	name := containerName(pkgPath, fuzzFuncName)
	// TODO: factor out
	// NB: guest workdir path.
	workdir := filepath.Join(".", "fleece", "workdirs", fuzzFuncName)
	runArgs := []string{
		"--rm", "-d",
		"--name", name,
		"--entrypoint", "/go-fuzz.sh",
		"-v", fmt.Sprintf("%s:/tmp/fuzzing", repoRoot),
		"go-fuzz", pkgPath, fuzzFuncName, build,
		"--", "-procs", fmt.Sprint(procs), "-workdir", workdir,
	}
	//fmt.Printf("runArgs: %s\n", strings.Join(runArgs, " "))

	if err := runContainer(repoRoot, runArgs); err != nil {
		return err
	}

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, os.Interrupt, os.Kill)

	ctx, cancel := context.WithCancel(context.Background())
	// TODO: use docker engine api
	go func() {
		logArgs := []string{"logs", "-f", name}
		logCmd := exec.Command("docker", logArgs...)
		logCmd.Stdout = os.Stdout
		logCmd.Stderr = os.Stderr
		if err := logCmd.Start(); err != nil {
			panic(err)
		}
		for {
			select {
			case <-ctx.Done():
				if err := logCmd.Process.Kill(); err != nil {
					panic(err)
				}
			default:
			}
		}
	}()

	// Wait for interrupt / kill signal
	_ = <-sigC
	cancel()

	// NB: false hope that progress is happening
	fmt.Printf("Shutting down gracefully...")
	go func() {
		for range time.Tick(1 * time.Second) {
			fmt.Print(".")
		}
	}()

	if err := stopContainer(name); err != nil {
		return fmt.Errorf("error encountered while stopping container: %w", err)
	}
	return nil
}

func containerName(pkgPath, fuzzFuncName string) string {
	return fmt.Sprintf("%s_%s", filepath.Base(pkgPath), fuzzFuncName)
}

// TODO: move to docker package
func runContainer(cwd string, args []string) error {
	args = append([]string{"run"}, args...)
	dockerCmd := exec.Command("docker", args...)
	dockerCmd.Dir = cwd
	dockerCmd.Stdout = os.Stdout
	dockerCmd.Stderr = os.Stderr
	return dockerCmd.Run()
}

// TODO: move to docker package
func stopContainer(name string) error {
	cmd := exec.Command("docker", "stop", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}