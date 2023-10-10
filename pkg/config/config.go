package config

import (
	"fmt"
	"github.com/spf13/pflag"
	"strings"
)

type Config struct {
	BuildToolchain bool
	Ldflags        string
	Asmflags       string
	Gcflags        string
	Output         string
	Parallel       int
	Tags           string
	Cgo            bool
	Rebuild        bool
	Race           bool
	GoCmd          string
	ModMode        string
	PlatformFlag   PlatformFlag
}

type PlatformFlag struct {
	OS     []string
	Arch   []string
	OSArch []Platform
	All    bool
}

func (p *PlatformFlag) Platforms(supported []Platform) []Platform {
	ignoreArch := make(map[string]struct{})
	includeArch := make(map[string]struct{})
	ignoreOS := make(map[string]struct{})
	includeOS := make(map[string]struct{})
	ignoreOSArch := make(map[string]Platform)
	includeOSArch := make(map[string]Platform)
	for _, v := range p.Arch {
		if v[0] == '!' {
			ignoreArch[v[1:]] = struct{}{}
		} else {
			includeArch[v] = struct{}{}
		}
	}
	for _, v := range p.OS {
		if v[0] == '!' {
			ignoreOS[v[1:]] = struct{}{}
		} else {
			includeOS[v] = struct{}{}
		}
	}
	for _, v := range p.OSArch {
		if v.OS[0] == '!' {
			v = Platform{
				OS:   v.OS[1:],
				Arch: v.Arch,
			}

			ignoreOSArch[v.String()] = v
		} else {
			includeOSArch[v.String()] = v
		}
	}

	// We're building a list of new platforms, so build the list
	// based only on the configured OS/arch pairs.
	var prefilter []Platform = nil
	if len(includeOSArch) > 0 {
		prefilter = make([]Platform, 0, len(p.Arch)*len(p.OS)+len(includeOSArch))
		for _, v := range includeOSArch {
			prefilter = append(prefilter, v)
		}
	}

	if len(includeOS) > 0 && len(includeArch) > 0 {
		// Build up the list of prefiltered by what is specified
		if prefilter == nil {
			prefilter = make([]Platform, 0, len(p.Arch)*len(p.OS))
		}

		for _, os := range p.OS {
			if _, ok := includeOS[os]; !ok {
				continue
			}

			for _, arch := range p.Arch {
				if _, ok := includeArch[arch]; !ok {
					continue
				}

				prefilter = append(prefilter, Platform{
					OS:   os,
					Arch: arch,
				})
			}
		}
	} else if len(includeOS) > 0 {
		// Build up the list of prefiltered by what is specified
		if prefilter == nil {
			prefilter = make([]Platform, 0, len(p.Arch)*len(p.OS))
		}

		for _, os := range p.OS {
			for _, platform := range supported {
				if platform.OS == os {
					prefilter = append(prefilter, platform)
				}
			}
		}
	}

	if prefilter != nil {
		// Remove any that aren't supported
		result := make([]Platform, 0, len(prefilter))
		for _, pending := range prefilter {
			found := false
			for _, platform := range supported {
				if pending.String() == platform.String() {
					found = true
					break
				}
			}

			if found {
				add := pending
				add.Default = false
				result = append(result, add)
			}
		}

		prefilter = result
	}

	if prefilter == nil {
		prefilter = make([]Platform, 0, len(supported))
		for _, v := range supported {
			if v.Default || p.All {
				add := v
				add.Default = false
				prefilter = append(prefilter, add)
			}
		}
	}

	// Go through each default platform and filter out the bad ones
	result := make([]Platform, 0, len(prefilter))
	for _, platform := range prefilter {
		if len(ignoreOSArch) > 0 {
			if _, ok := ignoreOSArch[platform.String()]; ok {
				continue
			}
		}

		// We only want to check the components (OS and Arch) if we didn't
		// specifically ask to include it via the osarch.
		checkComponents := true
		if len(includeOSArch) > 0 {
			if _, ok := includeOSArch[platform.String()]; ok {
				checkComponents = false
			}
		}

		if checkComponents {
			if len(ignoreArch) > 0 {
				if _, ok := ignoreArch[platform.Arch]; ok {
					continue
				}
			}
			if len(ignoreOS) > 0 {
				if _, ok := ignoreOS[platform.OS]; ok {
					continue
				}
			}
			if len(includeArch) > 0 {
				if _, ok := includeArch[platform.Arch]; !ok {
					continue
				}
			}
			if len(includeOS) > 0 {
				if _, ok := includeOS[platform.OS]; !ok {
					continue
				}
			}
		}

		result = append(result, platform)
	}

	return result
}

func (p *PlatformFlag) OSArchFlagValue() pflag.Value {
	return (*appendPlatformValue)(&p.OSArch)
}

// appendPlatformValue is a flag.Value that appends a full platform (os/arch)
// to a list where the values from space-separated lines. This is used to
// satisfy the --osarch flag.
type appendPlatformValue []Platform

func (s *appendPlatformValue) String() string {
	return ""
}

func (s *appendPlatformValue) Set(value string) error {
	if value == "" {
		return nil
	}

	for _, v := range strings.Split(value, " ") {
		parts := strings.Split(v, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid platform syntax: %s should be os/arch", v)
		}

		platform := Platform{
			OS:   strings.ToLower(parts[0]),
			Arch: strings.ToLower(parts[1]),
		}

		s.appendIfMissing(&platform)
	}

	return nil
}

func (s *appendPlatformValue) Type() string {
	return "osArchValue"
}

func (s *appendPlatformValue) appendIfMissing(value *Platform) {
	for _, existing := range *s {
		if existing == *value {
			return
		}
	}

	*s = append(*s, *value)
}
