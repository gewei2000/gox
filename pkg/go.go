package pkg

import (
	"bytes"
	"fmt"
	"github.com/mitchellh/gox/pkg/config"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"text/template"
)

type OutputTemplateData struct {
	Dir  string
	OS   string
	Arch string
}

func GoCrossCompile(cfg *config.Config, platform config.Platform, packagePath string) error {
	env := append(os.Environ(), "GOOS="+platform.OS, "GOARCH="+platform.Arch)

	// If we're building for our own platform, then enable cgo always. We
	// respect the CGO_ENABLED flag if that is explicitly set on the platform.
	cgo := cfg.Cgo
	if !cfg.Cgo && os.Getenv("CGO_ENABLED") != "0" {
		cgo = runtime.GOOS == platform.OS && runtime.GOARCH == platform.Arch
	}

	// If cgo is enabled then set that env var
	if cgo {
		env = append(env, "CGO_ENABLED=1")
	} else {
		env = append(env, "CGO_ENABLED=0")
	}

	var outputPath bytes.Buffer
	tpl, err := template.New("output").Parse(cfg.Output)
	if err != nil {
		return err
	}
	tplData := OutputTemplateData{
		Dir:  filepath.Base(packagePath),
		OS:   platform.OS,
		Arch: platform.Arch,
	}
	if err := tpl.Execute(&outputPath, &tplData); err != nil {
		return err
	}

	if platform.OS == "windows" {
		outputPath.WriteString(".exe")
	}

	// Determine the full path to the output so that we can change our
	// working directory when executing go build.
	outputPathReal := outputPath.String()
	outputPathReal, err = filepath.Abs(outputPathReal)
	if err != nil {
		return err
	}

	// Go prefixes the import directory with '_' when it is outside
	// the GOPATH.For this, we just drop it since we move to that
	// directory to build.
	chdir := ""
	if packagePath[0] == '_' {
		if runtime.GOOS == "windows" {
			re := regexp.MustCompile("^/([a-zA-Z])_/")
			chdir = re.ReplaceAllString(packagePath[1:], "$1:\\")
			chdir = strings.Replace(chdir, "/", "\\", -1)
		} else {
			chdir = packagePath[1:]
		}

		packagePath = ""
	}

	args := []string{"build"}
	if cfg.Rebuild {
		args = append(args, "-a")
	}
	if cfg.ModMode != "" {
		args = append(args, "-mod", cfg.ModMode)
	}
	if cfg.Race {
		args = append(args, "-race")
	}
	args = append(args,
		"-gcflags", cfg.Gcflags,
		"-ldflags", cfg.Ldflags,
		"-asmflags", cfg.Asmflags,
		"-tags", cfg.Tags,
		"-o", outputPathReal,
		packagePath)

	_, err = execGo(cfg.GoCmd, env, chdir, args...)
	return err
}

// GoMainDirs returns the file paths to the packages that are "main"
// packages, from the list of packages given. The list of packages can
// include relative paths, the special "..." Go keyword, etc.
func GoMainDirs(packages []string, GoCmd string) ([]string, error) {
	args := make([]string, 0, len(packages)+3)
	args = append(args, "list", "-f", "{{.Name}}|{{.ImportPath}}")
	args = append(args, packages...)

	output, err := execGo(GoCmd, nil, "", args...)
	if err != nil {
		return nil, err
	}

	results := make([]string, 0, len(output))
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			log.Printf("Bad line reading packages: %s", line)
			continue
		}

		if parts[0] == "main" {
			results = append(results, parts[1])
		}
	}

	return results, nil
}

// GoRoot returns the GOROOT value for the compiled `go` binary.
func GoRoot() (string, error) {
	output, err := execGo("go", nil, "", "env", "GOROOT")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(output), nil
}

func GoVersion() string {
	return runtime.Version()
}

func GoVersionParts() (result [2]int, err error) {
	version := GoVersion()
	_, err = fmt.Sscanf(version, "go%d.%d", &result[0], &result[1])
	return
}

func execGo(GoCmd string, env []string, dir string, args ...string) (string, error) {
	var stderr, stdout bytes.Buffer
	cmd := exec.Command(GoCmd, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if env != nil {
		cmd.Env = env
	}
	if dir != "" {
		cmd.Dir = dir
	}
	if err := cmd.Run(); err != nil {
		err = fmt.Errorf("%s\nStderr: %s", err, stderr.String())
		return "", err
	}

	return stdout.String(), nil
}
