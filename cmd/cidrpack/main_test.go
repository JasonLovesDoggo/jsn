package main

import (
	"math/big"
	"net/netip"
	"sort"
	"testing"
)

func parsePrefixes(ss []string) []netip.Prefix {
	var out []netip.Prefix
	for _, s := range ss {
		p, _ := netip.ParsePrefix(s)
		out = append(out, p.Masked())
	}
	return out
}

func prefixStrings(ps []netip.Prefix) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.String())
	}
	return out
}

func TestOptimize(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "empty",
			input: []string{},
			want:  nil,
		},
		{
			name:  "single",
			input: []string{"10.0.0.0/24"},
			want:  []string{"10.0.0.0/24"},
		},
		{
			name:  "adjacent merge",
			input: []string{"10.0.0.0/24", "10.0.1.0/24"},
			want:  []string{"10.0.0.0/23"},
		},
		{
			name:  "adjacent reverse order",
			input: []string{"10.0.1.0/24", "10.0.0.0/24"},
			want:  []string{"10.0.0.0/23"},
		},
		{
			name:  "subset removal",
			input: []string{"10.0.0.0/16", "10.0.1.0/24"},
			want:  []string{"10.0.0.0/16"},
		},
		{
			name:  "subset first",
			input: []string{"10.0.1.0/24", "10.0.0.0/16"},
			want:  []string{"10.0.0.0/16"},
		},
		{
			name:  "overlapping",
			input: []string{"10.0.0.0/24", "10.0.0.128/25"},
			want:  []string{"10.0.0.0/24"},
		},
		{
			name:  "four to one",
			input: []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"},
			want:  []string{"10.0.0.0/22"},
		},
		{
			name:  "non-mergeable gap",
			input: []string{"10.0.0.0/24", "10.0.2.0/24"},
			want:  []string{"10.0.0.0/24", "10.0.2.0/24"},
		},
		{
			name:  "unaligned range",
			input: []string{"10.0.0.1/32", "10.0.0.2/32", "10.0.0.3/32"},
			want:  []string{"10.0.0.1/32", "10.0.0.2/31"},
		},
		{
			name:  "full merge chain",
			input: []string{"192.168.0.0/25", "192.168.0.128/25"},
			want:  []string{"192.168.0.0/24"},
		},
		{
			name:  "duplicates",
			input: []string{"10.0.0.0/24", "10.0.0.0/24", "10.0.0.0/24"},
			want:  []string{"10.0.0.0/24"},
		},
		{
			name:  "all hosts in /30",
			input: []string{"10.0.0.0/32", "10.0.0.1/32", "10.0.0.2/32", "10.0.0.3/32"},
			want:  []string{"10.0.0.0/30"},
		},
		{
			name:  "partial overlap extend",
			input: []string{"10.0.0.0/25", "10.0.0.64/26", "10.0.0.128/25"},
			want:  []string{"10.0.0.0/24"},
		},
		{
			name:  "cascading merge",
			input: []string{"10.0.0.0/32", "10.0.0.1/32"},
			want:  []string{"10.0.0.0/31"},
		},
		{
			name:  "non-power-of-two count",
			input: []string{"10.0.0.0/32", "10.0.0.1/32", "10.0.0.2/32"},
			want:  []string{"10.0.0.0/31", "10.0.0.2/32"},
		},
		{
			name:  "whole internet v4",
			input: []string{"0.0.0.0/1", "128.0.0.0/1"},
			want:  []string{"0.0.0.0/0"},
		},
		{
			name:  "touching but not mergeable",
			input: []string{"10.0.1.0/24", "10.0.2.0/24"},
			want:  []string{"10.0.1.0/24", "10.0.2.0/24"},
		},
		{
			name:  "many scattered",
			input: []string{"10.0.0.0/24", "10.0.4.0/24", "10.0.8.0/24"},
			want:  []string{"10.0.0.0/24", "10.0.4.0/24", "10.0.8.0/24"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := optimize(parsePrefixes(tt.input), 32)
			gotStr := prefixStrings(got)

			if len(got) == 0 && len(tt.want) == 0 {
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", gotStr, tt.want)
				return
			}

			for i := range got {
				if got[i].String() != tt.want[i] {
					t.Errorf("got %v, want %v", gotStr, tt.want)
					return
				}
			}
		})
	}
}

func TestOptimizeIPv6(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "single v6",
			input: []string{"2001:db8::/32"},
			want:  []string{"2001:db8::/32"},
		},
		{
			name:  "adjacent v6",
			input: []string{"2001:db8::/33", "2001:db8:8000::/33"},
			want:  []string{"2001:db8::/32"},
		},
		{
			name:  "v6 subset",
			input: []string{"2001:db8::/32", "2001:db8:1::/48"},
			want:  []string{"2001:db8::/32"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := optimize(parsePrefixes(tt.input), 128)
			gotStr := prefixStrings(got)

			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", gotStr, tt.want)
				return
			}

			for i := range got {
				if got[i].String() != tt.want[i] {
					t.Errorf("got %v, want %v", gotStr, tt.want)
					return
				}
			}
		})
	}
}

// rangesEqual checks if two sets of prefixes cover the exact same IP space
func rangesEqual(a, b []netip.Prefix, bits int) bool {
	ar := prefixesToRanges(a, bits)
	br := prefixesToRanges(b, bits)
	if len(ar) != len(br) {
		return false
	}
	for i := range ar {
		if ar[i].start.Cmp(br[i].start) != 0 || ar[i].end.Cmp(br[i].end) != 0 {
			return false
		}
	}
	return true
}

func prefixesToRanges(prefixes []netip.Prefix, bits int) []ipRange {
	if len(prefixes) == 0 {
		return nil
	}
	ranges := make([]ipRange, len(prefixes))
	for i, p := range prefixes {
		ranges[i] = prefixToRange(p, bits)
	}
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].start.Cmp(ranges[j].start) < 0
	})
	merged := []ipRange{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		nextAfterLast := new(big.Int).Add(last.end, big.NewInt(1))
		if r.start.Cmp(nextAfterLast) <= 0 {
			if r.end.Cmp(last.end) > 0 {
				last.end = r.end
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

func TestOptimizePreservesIPs(t *testing.T) {
	tests := [][]string{
		{"10.0.0.0/28", "10.0.0.16/28"},
		{"10.0.0.0/30", "10.0.0.4/30", "10.0.0.8/30"},
		{"10.0.0.5/32", "10.0.0.6/32", "10.0.0.7/32"},
		{"10.0.0.0/28", "10.0.0.8/29"},
		{"192.168.0.0/26", "192.168.0.64/26", "192.168.0.128/26", "192.168.0.192/26"},
		{"10.0.0.0/8", "172.16.0.0/12"},
	}

	for _, input := range tests {
		prefixes := parsePrefixes(input)
		optimized := optimize(prefixes, 32)

		if !rangesEqual(prefixes, optimized, 32) {
			t.Errorf("IP coverage changed for input %v -> %v", input, prefixStrings(optimized))
		}
	}
}

func TestOptimizeReducesCount(t *testing.T) {
	tests := [][]string{
		{"10.0.0.0/24", "10.0.1.0/24"},
		{"10.0.0.0/24", "10.0.0.0/25"},
		{"10.0.0.0/32", "10.0.0.1/32", "10.0.0.2/32", "10.0.0.3/32"},
	}

	for _, input := range tests {
		prefixes := parsePrefixes(input)
		optimized := optimize(prefixes, 32)

		if len(optimized) > len(prefixes) {
			t.Errorf("optimization increased count: %v -> %v", input, prefixStrings(optimized))
		}
	}
}

func TestNoOverlapsInOutput(t *testing.T) {
	inputs := [][]string{
		{"10.0.0.0/24", "10.0.1.0/24", "10.0.0.128/25"},
		{"192.168.0.0/16", "192.168.1.0/24", "192.168.2.0/23"},
	}

	for _, input := range inputs {
		prefixes := parsePrefixes(input)
		optimized := optimize(prefixes, 32)

		for i := 0; i < len(optimized); i++ {
			for j := i + 1; j < len(optimized); j++ {
				a, b := optimized[i], optimized[j]
				if a.Overlaps(b) {
					t.Errorf("output has overlaps: %s and %s (input: %v)", a, b, input)
				}
			}
		}
	}
}

func TestOutputIsSorted(t *testing.T) {
	input := []string{"10.0.5.0/24", "10.0.1.0/24", "10.0.3.0/24"}
	prefixes := parsePrefixes(input)
	optimized := optimize(prefixes, 32)

	for i := 1; i < len(optimized); i++ {
		if optimized[i].Addr().Less(optimized[i-1].Addr()) {
			t.Errorf("output not sorted: %v", prefixStrings(optimized))
		}
	}
}

func FuzzOptimize(f *testing.F) {
	f.Add([]byte{10, 0, 0, 0, 24, 10, 0, 1, 0, 24})
	f.Add([]byte{192, 168, 0, 0, 16})
	f.Add([]byte{0, 0, 0, 0, 0})
	f.Add([]byte{255, 255, 255, 255, 32})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 5 || len(data)%5 != 0 {
			return
		}

		var prefixes []netip.Prefix
		for i := 0; i+5 <= len(data); i += 5 {
			bits := int(data[i+4] % 33)
			if bits == 0 {
				bits = 1
			}
			addr := netip.AddrFrom4([4]byte{data[i], data[i+1], data[i+2], data[i+3]})
			prefix, err := addr.Prefix(bits)
			if err != nil {
				continue
			}
			prefixes = append(prefixes, prefix.Masked())
		}

		if len(prefixes) == 0 {
			return
		}

		optimized := optimize(prefixes, 32)

		if !rangesEqual(prefixes, optimized, 32) {
			t.Errorf("IP coverage changed: input=%v output=%v",
				prefixStrings(prefixes), prefixStrings(optimized))
		}

		for i := 0; i < len(optimized); i++ {
			for j := i + 1; j < len(optimized); j++ {
				if optimized[i].Overlaps(optimized[j]) {
					t.Errorf("overlapping output: %s %s", optimized[i], optimized[j])
				}
			}
		}

		if len(optimized) > len(prefixes) {
			t.Errorf("output larger than input")
		}
	})
}

func FuzzRoundtrip(f *testing.F) {
	f.Add(uint8(10), uint8(0), uint8(0), uint8(0), uint8(28))
	f.Add(uint8(192), uint8(168), uint8(1), uint8(0), uint8(26))

	f.Fuzz(func(t *testing.T, a, b, c, d, bits uint8) {
		if bits > 32 || bits < 26 {
			return
		}

		base := netip.AddrFrom4([4]byte{a, b, c, d})
		prefix, err := base.Prefix(int(bits))
		if err != nil {
			return
		}
		prefix = prefix.Masked()

		r := prefixToRange(prefix, 32)
		size := new(big.Int).Sub(r.end, r.start)
		size.Add(size, big.NewInt(1))

		var singles []netip.Prefix
		for i := new(big.Int).Set(r.start); i.Cmp(r.end) <= 0; i.Add(i, big.NewInt(1)) {
			addr := intToAddr(i, 32)
			p, _ := addr.Prefix(32)
			singles = append(singles, p)
		}

		optimized := optimize(singles, 32)

		if len(optimized) != 1 || optimized[0] != prefix {
			t.Errorf("expanding %s to /32s and optimizing gave %v, want [%s]",
				prefix, prefixStrings(optimized), prefix)
		}
	})
}

func BenchmarkOptimize100(b *testing.B) {
	var prefixes []netip.Prefix
	for i := 0; i < 100; i++ {
		addr := netip.AddrFrom4([4]byte{10, byte(i / 256), byte(i % 256), 0})
		p, _ := addr.Prefix(24)
		prefixes = append(prefixes, p)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		optimize(prefixes, 32)
	}
}

func BenchmarkOptimize1000(b *testing.B) {
	var prefixes []netip.Prefix
	for i := 0; i < 1000; i++ {
		addr := netip.AddrFrom4([4]byte{10, byte(i / 256), byte(i % 256), 0})
		p, _ := addr.Prefix(24)
		prefixes = append(prefixes, p)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		optimize(prefixes, 32)
	}
}
