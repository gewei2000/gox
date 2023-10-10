package cmd

import (
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/mitchellh/gox/pkg"
	"github.com/mitchellh/gox/pkg/config"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

var cfg = &config.Config{
	BuildToolchain: false,
	Ldflags:        "",
	Output:         "",
	Parallel:       -1,
	Tags:           "",
	Gcflags:        "",
	Asmflags:       "",
	Cgo:            false,
	Rebuild:        false,
	Race:           false,
	GoCmd:          "go",
	ModMode:        "mod",
	PlatformFlag: config.PlatformFlag{
		OS:     nil,
		Arch:   nil,
		OSArch: nil,
		All:    false,
	},
}

var rootCmd = &cobra.Command{
	Use:   "gox",
	Short: "cross-compiles go applications in parallel.",
	Long:  helpText,
	Run: func(cmd *cobra.Command, args []string) {
		main(args, cfg)
	},
}

func main(args []string, cfg *config.Config) int {
	if cfg.Parallel < 2 {
		cpus := runtime.NumCPU()
		if cpus != 1 {
			cfg.Parallel = cpus - 1
		} else {
			cfg.Parallel = cpus
		}
	}

	if cfg.BuildToolchain {
		return pkg.BuildToolchain(cfg, cfg.PlatformFlag)
	}

	if _, err := exec.LookPath(cfg.GoCmd); err != nil {
		fmt.Fprintf(os.Stderr, "%s executable must be on the PATH\n", cfg.GoCmd)
		return 1
	}

	packages := args
	if len(packages) == 0 {
		packages = []string{"."}
	}

	// Get the packages that are in the given paths
	mainDirs, err := pkg.GoMainDirs(packages, cfg.GoCmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading packages: %s", err)
		return 1
	}

	// Determine the platforms we're building for
	platforms := cfg.PlatformFlag.Platforms(config.SupportedPlatforms())
	if len(platforms) == 0 {
		fmt.Println("No valid platforms to build for. If you specified a value")
		fmt.Println("for the 'os', 'arch', or 'osarch' flags, make sure you're")
		fmt.Println("using a valid value.")
		return 1
	}

	versionStr := pkg.GoVersion()
	// Assume -mod is supported when no version prefix is found
	if cfg.ModMode != "" && strings.HasPrefix(versionStr, "go") {
		// go-version only cares about version numbers
		current, err := version.NewVersion(versionStr[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to parse current go version: %s\n%s", versionStr, err.Error())
			return 1
		}

		constraint, err := version.NewConstraint(">= 1.11")
		if err != nil {
			panic(err)
		}

		if !constraint.Check(current) {
			fmt.Printf("Go compiler version %s does not support the -mod flag\n", versionStr)
			cfg.ModMode = ""
		}
	}

	// Build in parallel!
	fmt.Printf("Number of parallel builds: %d\n\n", cfg.Parallel)
	var errorLock sync.Mutex
	var wg sync.WaitGroup
	errors := make([]string, 0)
	semaphore := make(chan int, cfg.Parallel)
	for _, platform := range platforms {
		for _, path := range mainDirs {
			// Start the goroutine that will do the actual build
			wg.Add(1)
			go func(path string, platform config.Platform) {
				defer wg.Done()
				semaphore <- 1
				fmt.Printf("--> %15s: %s\n", platform.String(), path)

				// Determine if we have specific CFLAGS or LDFLAGS for this
				// GOOS/GOARCH combo and override the defaults if so.
				envOverride(&cfg.Ldflags, platform, "LDFLAGS")
				envOverride(&cfg.Gcflags, platform, "GCFLAGS")
				envOverride(&cfg.Asmflags, platform, "ASMFLAGS")

				if err := pkg.GoCrossCompile(cfg, platform, path); err != nil {
					errorLock.Lock()
					defer errorLock.Unlock()
					errors = append(errors,
						fmt.Sprintf("%s error: %s", platform.String(), err))
				}
				<-semaphore
			}(path, platform)
		}
	}
	wg.Wait()

	if len(errors) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d errors occurred:\n", len(errors))
		for _, err := range errors {
			fmt.Fprintf(os.Stderr, "--> %s\n", err)
		}
		return 1
	}

	return 0
}

func envOverride(target *string, platform config.Platform, key string) {
	key = strings.ToUpper(fmt.Sprintf(
		"GOX_%s_%s_%s", platform.OS, platform.Arch, key))
	if v := os.Getenv(key); v != "" {
		*target = v
	}
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

const helpText = `Usage: gox [options] [packages]

  Gox cross-compiles Go applications in parallel.

  If no specific operating systems or architectures are specified, Gox
  will build for all pairs supported by your version of Go.

Output path template:

  The output path for the compiled binaries is specified with the
  "--output" flag. The value is a string that is a Go text template.
  The default value is "{{.Dir}}_{{.OS}}_{{.Arch}}". The variables and
  their values should be self-explanatory.

Platforms (OS/Arch):

  The operating systems and architectures to cross-compile for may be
  specified with the "--arch" and "--os" flags. These are space separated lists
  of valid GOOS/GOARCH values to build for, respectively. You may prefix an
  OS or Arch with "!" to negate and not build for that platform. If the list
  is made up of only negations, then the negations will come from the default
  list.

  Additionally, the "--osarch" flag may be used to specify complete os/arch
  pairs that should be built or ignored. The syntax for this is what you would
  expect: "darwin/amd64" would be a valid osarch value. Multiple can be space
  separated. An os/arch pair can begin with "!" to not build for that platform.

  The "--osarch" flag has the highest precedent when determing whether to
  build for a platform. If it is included in the "--osarch" list, it will be
  built even if the specific os and arch is negated in "--os" and "--arch",
  respectively.

Platform Overrides:

  The "--gcflags", "--ldflags" and "--asmflags" options can be overridden per-platform
  by using environment variables. Gox will look for environment variables
  in the following format and use those to override values if they exist:

    GOX_[OS]_[ARCH]_GCFLAGS
    GOX_[OS]_[ARCH]_LDFLAGS
    GOX_[OS]_[ARCH]_ASMFLAGS

`

func init() {
	rootCmd.Flags().SortFlags = false

	rootCmd.Flags().StringSliceVar(&cfg.PlatformFlag.OS, "os", nil, "os to build for or skip")
	rootCmd.Flags().StringSliceVar(&cfg.PlatformFlag.Arch, "arch", nil, "arch to build for or skip")
	rootCmd.Flags().Var(cfg.PlatformFlag.OSArchFlagValue(), "osarch", "os/arch pairs to build for or skip")
	rootCmd.Flags().BoolVar(&cfg.PlatformFlag.All, "all", false, "build all supported platforms")

	rootCmd.Flags().StringVar(&cfg.Tags, "tags", "", "go build tags")
	rootCmd.Flags().StringVar(&cfg.Output, "output", "{{.Dir}}_{{.OS}}_{{.Arch}}", "output path")

	rootCmd.Flags().IntVar(&cfg.Parallel, "parallel", -1, "amount of parallelism, defaults to number of cpus")
	rootCmd.Flags().BoolVar(&cfg.BuildToolchain, "build-toolchain", false, "build cross-compilation toolchain")
	rootCmd.Flags().BoolVar(&cfg.Cgo, "cgo", false, "sets cgo_enabled=1, requires proper c toolchain (advanced)")
	rootCmd.Flags().BoolVar(&cfg.Rebuild, "rebuild", false, "force rebuilding of package that were up to date")
	rootCmd.Flags().BoolVar(&cfg.Race, "race", false, "build with the go race detector enabled, requires cgo")

	rootCmd.Flags().StringVar(&cfg.Ldflags, "ldflags", "", "linker flags")
	rootCmd.Flags().StringVar(&cfg.Gcflags, "gcflags", "", "gcflags, eg:all=-trimpath=${GOPATH}")
	rootCmd.Flags().StringVar(&cfg.Asmflags, "asmflags", "", "asmflags, eg:all=-trimpath=${GOPATH}")
	rootCmd.Flags().StringVar(&cfg.GoCmd, "gocmd", "go", "go cmd")
	rootCmd.Flags().StringVar(&cfg.ModMode, "mod", "", "go mod mode")

}
