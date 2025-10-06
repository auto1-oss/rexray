//go:build !windows
// +build !windows

// ABOUTME: Unix-specific helpers for device discovery and NVMe detection.
// ABOUTME: Provides device range utilities for EBS integrations on Linux.
package utils

import (
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/rexray/rexray/libstorage/api/types"
)

// DeviceRange holds slices for device namespace iteration
// and patterns for matching device nodes to specific namespaces.
//
// EBS suggests to use /dev/sd[f-p] for Linux EC2 instances. Also on Linux EC2
// instances, although the device path may show up as /dev/sd* on the EC2
// side, it will appear locally as /dev/xvd*
//
// The broadest device path namespace available for Linux EC2 instances is
// /dev/xvd[b-c][a-z]
// See http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/device_naming.htm
type DeviceRange struct {
	ParentLetters  []string
	ChildLetters   []string
	NextDeviceInfo *types.NextDeviceInfo
	DeviceRE       *regexp.Regexp
}

var (
	largeDeviceRange = &DeviceRange{
		ParentLetters: []string{"b", "c"},
		ChildLetters: []string{
			"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m",
			"n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"},
		NextDeviceInfo: &types.NextDeviceInfo{
			Prefix:  "xvd",
			Pattern: "[b-c][a-z]",
			Ignore:  false,
		},
		DeviceRE: regexp.MustCompile(`^(?:xvd[b-c][a-z]?|nvme\d+n\d+(?:p\d+)?)$`),
	}
	defaultDeviceRange = &DeviceRange{
		ParentLetters: []string{"d"},
		ChildLetters: []string{
			"f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"},
		NextDeviceInfo: &types.NextDeviceInfo{
			Prefix:  "xvd",
			Pattern: "[f-p]",
			Ignore:  false,
		},
		DeviceRE: regexp.MustCompile(`^(?:xvd[d-f]?[f-p]|nvme\d+n\d+(?:p\d+)?)$`),
	}
)

var (
	nvmeHostOnce sync.Once
	nvmeHost     bool
)

func resetDeviceRange() {
	nvmeHostOnce = sync.Once{}
	nvmeHost = false
}

// IsNVMEHost returns true when NVMe devices are present on the system.
func IsNVMEHost(ctx types.Context) bool {
	nvmeHostOnce.Do(func() {
		if matches, err := filepath.Glob("/dev/nvme*n*"); err == nil && len(matches) > 0 {
			nvmeHost = true
			return
		}
		if _, err := os.Stat("/dev/nvme0"); err == nil {
			nvmeHost = true
		}
	})
	return nvmeHost
}

// GetDeviceRange returns a specified DeviceRange object
func GetDeviceRange(useLargeDeviceRange bool) *DeviceRange {
	if useLargeDeviceRange {
		return largeDeviceRange
	}
	return defaultDeviceRange
}
