package neutron

import (
	"regexp"
	"testing"
)

func TestGenerateMAC(t *testing.T) {
	t.Run("format validation", func(t *testing.T) {
		mac := generateMAC()
		pattern := regexp.MustCompile(`^[0-9a-f]{2}(:[0-9a-f]{2}){5}$`)
		if !pattern.MatchString(mac) {
			t.Errorf("MAC address %q does not match expected format XX:XX:XX:XX:XX:XX", mac)
		}
	})

	t.Run("local bit set and multicast cleared", func(t *testing.T) {
		// Run multiple times to increase confidence
		for i := 0; i < 100; i++ {
			mac := generateMAC()
			// Parse the first octet
			var first byte
			if _, err := parseHexByte(mac[0:2], &first); err != nil {
				t.Fatalf("failed to parse first octet of MAC %q: %v", mac, err)
			}
			// Local bit (bit 1) must be set
			if first&0x02 == 0 {
				t.Errorf("MAC %q: local bit (bit 1) is not set in first octet 0x%02x", mac, first)
			}
			// Multicast bit (bit 0) must be cleared
			if first&0x01 != 0 {
				t.Errorf("MAC %q: multicast bit (bit 0) is not cleared in first octet 0x%02x", mac, first)
			}
		}
	})

	t.Run("uniqueness over 100 iterations", func(t *testing.T) {
		seen := make(map[string]struct{}, 100)
		for i := 0; i < 100; i++ {
			mac := generateMAC()
			if _, exists := seen[mac]; exists {
				t.Errorf("duplicate MAC address generated: %q", mac)
			}
			seen[mac] = struct{}{}
		}
	})
}

// parseHexByte parses a 2-character hex string into a byte.
func parseHexByte(s string, out *byte) (int, error) {
	var n int
	_, err := func() (int, error) {
		var v uint8
		_, scanErr := twoHexDigits(s, &v)
		if scanErr != nil {
			return 0, scanErr
		}
		*out = v
		return 2, nil
	}()
	return n, err
}

// twoHexDigits converts a 2-char hex string to uint8.
func twoHexDigits(s string, out *uint8) (int, error) {
	if len(s) < 2 {
		return 0, &hexError{s}
	}
	hi, ok1 := hexVal(s[0])
	lo, ok2 := hexVal(s[1])
	if !ok1 || !ok2 {
		return 0, &hexError{s}
	}
	*out = hi<<4 | lo
	return 2, nil
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

type hexError struct{ s string }

func (e *hexError) Error() string { return "invalid hex string: " + e.s }
