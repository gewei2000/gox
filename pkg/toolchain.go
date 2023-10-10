package pkg

import (
	"bytes"
	"fmt"
	"github.com/mitchellh/gox/pkg/config"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

func BuildToolchain(cfg *config.Config, platformFlag config.PlatformFlag) int {
	if _, err := exec.LookPath("go"); err != nil {
		fmt.Fprintf(os.Stderr, "You must have Go already built for your native platform\n")
		fmt.Fprintf(os.Stderr, "and the `go` binary on the PATH to build toolchains.\n")
		return 1
	}

	// If we're version 1.5 or greater, then we don't need to do this anymore!
	versionParts, err := GoVersionParts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading Go version: %s", err)
		return 1
	}
	if versionParts[0] >= 1 && versionParts[1] >= 5 {
		fmt.Fprintf(os.Stderr, "--build-toolchain is no longer required for Go 1.5 or later.\nYou can start using Gox immediately!\n")
		return 1
	}

	root, err := GoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding GOROOT: %s\n", err)
		return 1
	}

	// Determine the platforms we're building the toolchain for.
	platforms := platformFlag.Platforms(config.SupportedPlatforms())

	// The toolchain build can't be parallelized.
	if cfg.Parallel > 1 {
		fmt.Println("The toolchain build can't be parallelized because compiling a single")
		fmt.Println("Go source directory can only be done for one platform at a time. Therefore,")
		fmt.Println("the toolchain for each platform will be built one at a time.\n ")
	}
	cfg.Parallel = 1

	var errorLock sync.Mutex
	var wg sync.WaitGroup
	errs := make([]error, 0)
	semaphore := make(chan int, cfg.Parallel)
	for _, platform := range platforms {
		wg.Add(1)
		go func(platform config.Platform) {
			err := buildToolchain(&wg, semaphore, root, platform)
			if err != nil {
				errorLock.Lock()
				defer errorLock.Unlock()
				errs = append(errs, fmt.Errorf("%s: %s", platform.String(), err))
			}
		}(platform)
	}
	wg.Wait()

	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d errors occurred:\n", len(errs))
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}
		return 1
	}

	return 0
}

func buildToolchain(wg *sync.WaitGroup, semaphore chan int, root string, platform config.Platform) error {
	defer wg.Done()
	semaphore <- 1
	defer func() { <-semaphore }()
	fmt.Printf("--> Toolchain: %s\n", platform.String())

	scriptName := "make.bash"
	if runtime.GOOS == "windows" {
		scriptName = "make.bat"
	}

	var stderr bytes.Buffer
	var stdout bytes.Buffer
	scriptDir := filepath.Join(root, "src")
	scriptPath := filepath.Join(scriptDir, scriptName)
	cmd := exec.Command(scriptPath, "--no-clean")
	cmd.Dir = scriptDir
	cmd.Env = append(os.Environ(),
		"GOARCH="+platform.Arch,
		"GOOS="+platform.OS)
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error building '%s': %s", platform.String(), err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error building '%s'.\n\nStdout: %s\n\nStderr: %s\n",
			platform.String(), stdout.String(), stderr.String())
	}

	return nil
}
