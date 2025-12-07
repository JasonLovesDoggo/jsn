package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/big"
	"net/netip"
	"os"
	"sort"
	"strings"
)

func main() {
	help := flag.Bool("h", false, "show help")
	flag.Parse()

	if *help {
		fmt.Fprintln(os.Stderr, "cidrpack - optimize CIDR blocks")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "usage: cidrpack < cidrs.txt")
		fmt.Fprintln(os.Stderr, "       echo '10.0.0.0/24 10.0.1.0/24' | cidrpack")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "reads CIDRs from stdin (space/newline separated)")
		fmt.Fprintln(os.Stderr, "outputs minimized non-overlapping CIDR list")
		os.Exit(0)
	}

	var v4, v6 []netip.Prefix

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, field := range strings.Fields(line) {
			prefix, err := netip.ParsePrefix(field)
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid cidr: %s\n", field)
				os.Exit(1)
			}
			prefix = prefix.Masked()
			if prefix.Addr().Is4() {
				v4 = append(v4, prefix)
			} else {
				v6 = append(v6, prefix)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	for _, p := range optimize(v4, 32) {
		fmt.Println(p)
	}
	for _, p := range optimize(v6, 128) {
		fmt.Println(p)
	}
}

type ipRange struct {
	start *big.Int
	end   *big.Int
}

func optimize(cidrs []netip.Prefix, bits int) []netip.Prefix {
	if len(cidrs) == 0 {
		return nil
	}

	ranges := make([]ipRange, len(cidrs))
	for i, c := range cidrs {
		ranges[i] = prefixToRange(c, bits)
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

	var result []netip.Prefix
	for _, r := range merged {
		result = append(result, rangeToPrefixes(r, bits)...)
	}
	return result
}

func prefixToRange(p netip.Prefix, maxBits int) ipRange {
	start := addrToInt(p.Addr())
	size := new(big.Int).Lsh(big.NewInt(1), uint(maxBits-p.Bits()))
	end := new(big.Int).Sub(new(big.Int).Add(start, size), big.NewInt(1))
	return ipRange{start, end}
}

func rangeToPrefixes(r ipRange, maxBits int) []netip.Prefix {
	var result []netip.Prefix
	current := new(big.Int).Set(r.start)

	for current.Cmp(r.end) <= 0 {
		trailingZeros := 0
		if current.Sign() != 0 {
			trailingZeros = trailingZeroBits(current)
		} else {
			trailingZeros = maxBits
		}

		remaining := new(big.Int).Sub(r.end, current)
		remaining.Add(remaining, big.NewInt(1))
		remainingBits := remaining.BitLen() - 1

		bits := min(trailingZeros, remainingBits)
		if bits > maxBits {
			bits = maxBits
		}

		prefixLen := maxBits - bits
		addr := intToAddr(current, maxBits)
		prefix, _ := addr.Prefix(prefixLen)
		result = append(result, prefix)

		blockSize := new(big.Int).Lsh(big.NewInt(1), uint(bits))
		current.Add(current, blockSize)
	}

	return result
}

func addrToInt(addr netip.Addr) *big.Int {
	b := addr.As16()
	if addr.Is4() {
		return new(big.Int).SetBytes(b[12:16])
	}
	return new(big.Int).SetBytes(b[:])
}

func intToAddr(i *big.Int, bits int) netip.Addr {
	b := i.Bytes()
	if bits == 32 {
		var arr [4]byte
		copy(arr[4-len(b):], b)
		return netip.AddrFrom4(arr)
	}
	var arr [16]byte
	copy(arr[16-len(b):], b)
	return netip.AddrFrom16(arr)
}

func trailingZeroBits(n *big.Int) int {
	if n.Sign() == 0 {
		return 0
	}
	count := 0
	tmp := new(big.Int).Set(n)
	for tmp.Bit(0) == 0 {
		count++
		tmp.Rsh(tmp, 1)
	}
	return count
}
