package endpoint

import (
	"net"
	"strconv"
	"testing"
)

// Test cases copied and modified from net.SplitHostPort.
func TestSplitAddressPort(t *testing.T) {
	for _, tt := range []struct {
		addressPort string
		address     string
		port        int
	}{
		// Host name
		{"localhost:80", "localhost", 80},

		// Go-specific host name with zone identifier
		{"localhost%lo0:80", "localhost%lo0", 80},
		{"[localhost%lo0]:80", "localhost%lo0", 80}, // Go 1 behavior

		// IP literal
		{"127.0.0.1:80", "127.0.0.1", 80},
		{"[::1]:80", "::1", 80},

		// IP literal with zone identifier
		{"[::1%lo0]:80", "::1%lo0", 80},

		// Go-specific wildcard for host name
		{":80", "", 80}, // Go 1 behavior
	} {
		if address, port, err := SplitAddressPort(tt.addressPort); address != tt.address || port != tt.port || err != nil {
			t.Errorf("SplitAddressPort(%q) = %q, %q, %v; want %q, %q, nil", tt.addressPort, address, port, err, tt.address, tt.port)
		}
	}

	for _, tt := range []struct {
		addressPort string
		err         string
	}{
		{"golang.org", "missing port in address"},
		{"127.0.0.1", "missing port in address"},
		{"[::1]", "missing port in address"},
		{"[fe80::1%lo0]", "missing port in address"},
		{"[localhost%lo0]", "missing port in address"},
		{"localhost%lo0", "missing port in address"},

		{"::1", "too many colons in address"},
		{"fe80::1%lo0", "too many colons in address"},
		{"fe80::1%lo0:80", "too many colons in address"},

		{"golang.org:", "strconv.Atoi: parsing \"\": invalid syntax"},
		{"127.0.0.1:", "strconv.Atoi: parsing \"\": invalid syntax"},
		{"[::1]:", "strconv.Atoi: parsing \"\": invalid syntax"},

		{"[foo:bar]", "missing port in address"},
		{"[foo:bar]baz", "missing port in address"},
		{"[foo]bar:baz", "missing port in address"},

		{"[foo]:[bar]:baz", "too many colons in address"},

		{"[foo]:[bar]baz", "unexpected '[' in address"},
		{"foo[bar]:baz", "unexpected '[' in address"},

		{"foo]bar:baz", "unexpected ']' in address"},
	} {
		if address, port, err := SplitAddressPort(tt.addressPort); err == nil {
			t.Errorf("SplitAddressPort(%q) should have failed", tt.addressPort)
		} else {
			//nolint:gosimple // Test taken from elsewhere.
			switch err.(type) {
			case *net.AddrError:
				e := err.(*net.AddrError)
				if e.Err != tt.err {
					t.Errorf("SplitAddressPort(%q) = _, _, %q; want %q", tt.addressPort, e.Err, tt.err)
				}
			case *strconv.NumError:
				if err.Error() != tt.err {
					t.Errorf("SplitAddressPort(%q) = _, _, %q; want %q", tt.addressPort, err.Error(), tt.err)
				}
			default:
				t.Errorf("SplitAddressPort(%q) = _, _, %q; want %q: Unexpected error type: %T", tt.addressPort, err.Error(), tt.err, err)
			}
			if address != "" || port != 0 {
				t.Errorf("SplitAddressPort(%q) = %q, %q, err; want %q, %q, err on failure", tt.addressPort, address, port, "", "")
			}
		}
	}
}
