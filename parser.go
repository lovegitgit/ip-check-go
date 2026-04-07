package ipcheck

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type parseMetrics struct {
	ResolveSeconds float64
	GeoSeconds     float64
	TotalSeconds   float64
}

func parseSources(ctx context.Context, cfg Config, geoSvc *geoService, enableGeo bool) ([]IPInfo, parseMetrics, error) {
	startTotal := time.Now()
	var raw []string
	for _, src := range cfg.Runtime.IPSources {
		lines, err := sourceLines(src)
		if err != nil {
			return nil, parseMetrics{}, err
		}
		raw = append(raw, lines...)
	}

	startResolve := time.Now()
	var direct []IPInfo
	var hosts []string
	seen := map[string]struct{}{}
	for _, item := range raw {
		if ctx.Err() != nil {
			return nil, parseMetrics{}, ctx.Err()
		}
		parsed := parseIPExpr(item, cfg)
		if len(parsed) > 0 {
			for _, ip := range parsed {
				key := fmt.Sprintf("%s|%d", ip.IP, ip.Port)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				direct = append(direct, ip)
			}
			continue
		}
		if isHostname(item) {
			hosts = append(hosts, item)
		}
	}

	resolved, err := resolveHostnames(ctx, uniqueStrings(hosts), cfg)
	if err != nil {
		return nil, parseMetrics{}, err
	}
	for _, ip := range resolved {
		key := fmt.Sprintf("%s|%d", ip.IP, ip.Port)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		direct = append(direct, ip)
	}
	resolveSeconds := round2(time.Since(startResolve).Seconds())

	startGeo := time.Now()
	if enableGeo && geoSvc != nil {
		direct = geoSvc.fill(ctx, direct)
	}
	geoSeconds := round2(time.Since(startGeo).Seconds())

	return direct, parseMetrics{
		ResolveSeconds: resolveSeconds,
		GeoSeconds:     geoSeconds,
		TotalSeconds:   round2(time.Since(startTotal).Seconds()),
	}, nil
}

func parseIPExpr(arg string, cfg Config) []IPInfo {
	if ips := parseBareIP(arg, cfg); len(ips) > 0 {
		return ips
	}
	if ips := parseCIDR(arg, cfg); len(ips) > 0 {
		return ips
	}
	if ips := parseIPPort(arg, cfg); len(ips) > 0 {
		return ips
	}
	return nil
}

func parseBareIP(arg string, cfg Config) []IPInfo {
	ip := strings.TrimPrefix(strings.TrimSuffix(arg, "]"), "[")
	if !isIPAddress(ip) || !addrAllowedByWhiteBlock(ip, cfg) || !addrAllowedByFamily(ip, cfg) {
		return nil
	}
	return []IPInfo{newIPInfo(ip, cfg.IPPort)}
}

func parseCIDR(arg string, cfg Config) []IPInfo {
	prefix, err := netip.ParsePrefix(arg)
	if err != nil || !addrAllowedByFamily(prefix.Addr().String(), cfg) {
		return nil
	}
	ips := samplePrefix(prefix, cfg.CIDRSampleIPNum)
	out := make([]IPInfo, 0, len(ips))
	for _, ip := range ips {
		if addrAllowedByWhiteBlock(ip, cfg) {
			out = append(out, newIPInfo(ip, cfg.IPPort))
		}
	}
	return out
}

func parseIPPort(arg string, cfg Config) []IPInfo {
	host, portStr, err := net.SplitHostPort(arg)
	if err != nil {
		if strings.Count(arg, ":") == 1 && !strings.Contains(arg, "]") {
			parts := strings.Split(arg, ":")
			host, portStr = parts[0], parts[1]
		} else {
			return nil
		}
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	port, err := strconv.Atoi(portStr)
	if err != nil || !portAllowed(port, cfg) || !isIPAddress(host) || !addrAllowedByWhiteBlock(host, cfg) || !addrAllowedByFamily(host, cfg) {
		return nil
	}
	return []IPInfo{newIPInfo(host, port)}
}

func resolveHostnames(ctx context.Context, hosts []string, cfg Config) ([]IPInfo, error) {
	if len(hosts) == 0 {
		return nil, nil
	}
	workerCount := minInt(max(1, cfg.Valid.ThreadNum), len(hosts))
	jobs := make(chan string)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var out []IPInfo
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case host, ok := <-jobs:
					if !ok {
						return
					}
					ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
					if err != nil {
						continue
					}
					resolved := make([]IPInfo, 0, len(ips))
					for _, ip := range ips {
						ipStr := ip.String()
						if !addrAllowedByWhiteBlock(ipStr, cfg) || !addrAllowedByFamily(ipStr, cfg) {
							continue
						}
						info := newIPInfo(ipStr, cfg.IPPort)
						info.Hostname = host
						resolved = append(resolved, info)
					}
					if len(resolved) == 0 {
						continue
					}
					mu.Lock()
					out = append(out, resolved...)
					mu.Unlock()
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, host := range hosts {
			select {
			case <-ctx.Done():
				return
			case jobs <- host:
			}
		}
	}()
	wg.Wait()
	slices.SortFunc(out, func(a, b IPInfo) int {
		if a.IP == b.IP {
			return strings.Compare(a.Hostname, b.Hostname)
		}
		return strings.Compare(a.IP, b.IP)
	})
	return out, nil
}
