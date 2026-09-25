package chroot

import "testing"

// the vendor field is the only part of a CHOST that may differ between a client
// and the server: same arch + same os/libc is the same target
func TestChostTarget(t *testing.T) {
	cases := []struct {
		chost string
		arch  string
		tail  string
	}{
		{"x86_64-pc-linux-gnu", "x86_64", "linux-gnu"},
		{"x86_64-unknown-linux-gnu", "x86_64", "linux-gnu"},
		{"x86_64-linux-gnu", "x86_64", "linux-gnu"},
		{"x86_64-pc-linux-musl", "x86_64", "linux-musl"},
		{"aarch64-unknown-linux-gnu", "aarch64", "linux-gnu"},
		{"x86_64", "x86_64", ""},
	}
	for _, c := range cases {
		arch, tail := chostTarget(c.chost)
		if arch != c.arch || tail != c.tail {
			t.Errorf("chostTarget(%q) = %q, %q, want %q, %q", c.chost, arch, tail, c.arch, c.tail)
		}
	}
}

// a native flavor serves the server's own CHOST: the vendor may differ, the arch
// and the os/libc may not. "no-such-flavor" has no cross.conf, so it takes the
// native branch
func TestArchRefusalNative(t *testing.T) {
	serverGccMachine = "x86_64-pc-linux-gnu"
	defer func() { serverGccMachine = "" }()

	for _, chost := range []string{"x86_64-pc-linux-gnu", "x86_64-unknown-linux-gnu", "x86_64-linux-gnu"} {
		if refusal := ArchRefusal("no-such-flavor", chost); refusal != "" {
			t.Errorf("ArchRefusal(%q) = %q, want no refusal", chost, refusal)
		}
	}
	for _, chost := range []string{"x86_64-pc-linux-musl", "aarch64-unknown-linux-gnu", "x86_64-apple-darwin"} {
		if refusal := ArchRefusal("no-such-flavor", chost); refusal == "" {
			t.Errorf("ArchRefusal(%q) = no refusal, want one", chost)
		}
	}
	// nothing to compare is not a mismatch
	if refusal := ArchRefusal("no-such-flavor", ""); refusal != "" {
		t.Errorf("ArchRefusal(\"\") = %q, want no refusal", refusal)
	}
}
