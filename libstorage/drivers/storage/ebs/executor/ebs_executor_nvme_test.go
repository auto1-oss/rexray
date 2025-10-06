// ABOUTME: Tests NVMe alias extraction logic for the EBS executor.
// ABOUTME: Ensures raw controller output maps to expected device aliases.
package executor

import "testing"

func TestExtractNVMEAlias(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "alias with dev prefix",
			data: []byte("\x00random\x00/dev/xvdf\x00tail"),
			want: "/dev/xvdf",
		},
		{
			name: "alias without prefix",
			data: []byte("ignored bytes xvdbm more bytes"),
			want: "/dev/xvdbm",
		},
		{
			name: "no alias present",
			data: []byte("nothing helpful here"),
			want: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractNVMEAlias(tc.data); got != tc.want {
				t.Fatalf("extractNVMEAlias() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSelectNVMEAlias(t *testing.T) {
	testCases := []struct {
		name string
		raw  []byte
		want string
	}{
		{
			name: "prefers extracted alias",
			raw: func() []byte {
				raw := make([]byte, 4000)
				copy(raw[200:], []byte("random data /dev/xvdf additional"))
				copy(raw[3072:], []byte("vol-0123456789abcdef"))
				return raw
			}(),
			want: "/dev/xvdf",
		},
		{
			name: "ignores volume identifier alias",
			raw: func() []byte {
				raw := make([]byte, 3104)
				copy(raw[3072:], []byte("vol-1234567890abcdef"))
				return raw
			}(),
			want: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := selectNVMEAlias(tc.raw); got != tc.want {
				t.Fatalf("selectNVMEAlias() = %q, want %q", got, tc.want)
			}
		})
	}
}
