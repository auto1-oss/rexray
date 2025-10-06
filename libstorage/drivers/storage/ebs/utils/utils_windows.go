//go:build windows
// +build windows

// ABOUTME: Windows stubs for EBS metadata helpers that have Unix implementations.
// ABOUTME: Provides no-op implementations for non-Unix builds.

package utils

import "github.com/rexray/rexray/libstorage/api/types"

func resetDeviceRange() {}

// IsNVMEHost always returns false on Windows.
func IsNVMEHost(ctx types.Context) bool { return false }
