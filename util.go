package ipcheck

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	mrand "math/rand"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var seededRand = mrand.New(mrand.NewSource(time.Now().UnixNano()))

func isIPAddress(s string) bool {
	_, err := netip.ParseAddr(strings.Trim(s, "[]"))
	return err == nil
}

func isIPNetwork(s string) bool {
	_, err := netip.ParsePrefix(s)
	return err == nil
}

func isHostname(s string) bool {
	if strings.Contains(s, "/") || strings.Contains(s, " ") {
		return false
	}
	if ip := strings.Trim(s, "[]"); isIPAddress(ip) {
		return false
	}
	return strings.Count(s, ".") >= 1
}

func ipVersion(s string) int {
	addr, err := netip.ParseAddr(strings.Trim(s, "[]"))
	if err != nil {
		return 0
	}
	if addr.Is4() {
		return 4
	}
	return 6
}

func addrAllowedByFamily(s string, cfg Config) bool {
	if cfg.Runtime.OnlyV4 == cfg.Runtime.OnlyV6 {
		return true
	}
	version := ipVersion(s)
	if cfg.Runtime.OnlyV4 {
		return version == 4
	}
	return version == 6
}

func addrAllowedByWhiteBlock(s string, cfg Config) bool {
	if len(cfg.Runtime.WhiteList) > 0 {
		for _, prefix := range cfg.Runtime.WhiteList {
			if strings.HasPrefix(s, prefix) {
				return true
			}
		}
		return false
	}
	for _, prefix := range cfg.Runtime.BlockList {
		if strings.HasPrefix(s, prefix) {
			return false
		}
	}
	return true
}

func portAllowed(port int, cfg Config) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	if len(cfg.Runtime.PreferPorts) == 0 {
		return true
	}
	for _, p := range cfg.Runtime.PreferPorts {
		if p == port {
			return true
		}
	}
	return false
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, item := range in {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func chooseUserAgent(enabled bool) string {
	if !enabled || len(userAgents) == 0 {
		return ""
	}
	return userAgents[seededRand.Intn(len(userAgents))]
}

func generatedTimeDescription() string {
	return "生成时间为: " + time.Now().Format("2006-01-02 15:04:05")
}

func sampleIPInfos(in []IPInfo, n int) []IPInfo {
	if n <= 0 || len(in) <= n {
		return in
	}
	indexes := seededRand.Perm(len(in))[:n]
	out := make([]IPInfo, 0, n)
	for _, idx := range indexes {
		out = append(out, in[idx])
	}
	return out
}

func samplePrefix(prefix netip.Prefix, size int) []string {
	if size <= 0 {
		return nil
	}
	addr := prefix.Masked().Addr()
	bits := addr.BitLen()
	hostBits := bits - prefix.Bits()
	if hostBits <= 0 {
		return []string{addr.String()}
	}
	max := new(big.Int).Lsh(big.NewInt(1), uint(hostBits))
	want := minInt(size, estimateSampleCap(max))
	results := make(map[string]struct{}, want)
	base := addr.AsSlice()
	for len(results) < want {
		offset := randomBigInt(max)
		ipBytes := addOffset(base, offset)
		ip, ok := netip.AddrFromSlice(ipBytes)
		if !ok {
			continue
		}
		results[ip.String()] = struct{}{}
	}
	out := make([]string, 0, len(results))
	for ip := range results {
		out = append(out, ip)
	}
	sort.Strings(out)
	return out
}

func estimateSampleCap(max *big.Int) int {
	if max.IsInt64() {
		v := max.Int64()
		if v < 1 {
			return 1
		}
		if v > int64(^uint(0)>>1) {
			return int(^uint(0) >> 1)
		}
		return int(v)
	}
	return int(^uint(0) >> 1)
}

func randomBigInt(max *big.Int) *big.Int {
	n, err := rand.Int(rand.Reader, max)
	if err == nil {
		return n
	}
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, seededRand.Uint64())
	return new(big.Int).SetBytes(buf)
}

func addOffset(base []byte, offset *big.Int) []byte {
	out := make([]byte, len(base))
	copy(out, base)
	offBytes := offset.Bytes()
	i := len(out) - 1
	j := len(offBytes) - 1
	carry := 0
	for i >= 0 {
		val := int(out[i]) + carry
		if j >= 0 {
			val += int(offBytes[j])
			j--
		}
		out[i] = byte(val & 0xff)
		carry = val >> 8
		i--
	}
	return out
}

func sourceLines(arg string) ([]string, error) {
	if st, err := os.Stat(arg); err == nil && !st.IsDir() {
		file, err := os.Open(arg)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		var out []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			out = append(out, line)
		}
		return out, scanner.Err()
	}
	return []string{arg}, nil
}

func defaultOutputPath(first string, port int) string {
	name := first
	if isIPNetwork(first) {
		name = strings.ReplaceAll(first, "/", "@")
	}
	dir := filepath.Dir(name)
	base := filepath.Base(name)
	base = strings.TrimSuffix(base, ".txt")
	base = strings.ReplaceAll(base, "::", "--")
	base = strings.ReplaceAll(base, ":", "-")
	if base == "." || base == "" {
		base = filepath.Base(dir)
	}
	return filepath.Join(dir, fmt.Sprintf("result_%s_%d.txt", base, port))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
