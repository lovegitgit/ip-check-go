package ipcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/oschwald/geoip2-golang/v2"
)

type geoService struct {
	cityReader *geoip2.Reader
	asnReader  *geoip2.Reader
	locAvailable bool
	asnAvailable bool
}

func openGeoService(paths appPaths) (*geoService, error) {
	var svc geoService
	if reader, err := geoip2.Open(paths.geoCityDB); err == nil {
		svc.cityReader = reader
		svc.locAvailable = true
	}
	if reader, err := geoip2.Open(paths.geoASNDB); err == nil {
		svc.asnReader = reader
		svc.asnAvailable = true
	}
	return &svc, nil
}

func (g *geoService) Close() {
	if g == nil {
		return
	}
	if g.cityReader != nil {
		_ = g.cityReader.Close()
	}
	if g.asnReader != nil {
		_ = g.asnReader.Close()
	}
}

func (g *geoService) fill(ctx context.Context, infos []IPInfo) []IPInfo {
	if g == nil {
		return infos
	}
	if g.cityReader == nil {
		consolePrint(fmt.Sprintf("%s 数据库异常, 无法获取IP 归属地信息, 请执行igeo-dl 重新下载!", geoCityDBName))
	}
	if g.asnReader == nil {
		consolePrint(fmt.Sprintf("%s 数据库异常, 无法获取IP ASN/ORG信息, 请执行igeo-dl 重新下载!", geoASNDBName))
	}
	out := make([]IPInfo, len(infos))
	type job struct {
		idx  int
		info IPInfo
	}
	workerCount := minInt(max(1, runtime.GOMAXPROCS(0)), len(infos))
	jobs := make(chan job)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				out[item.idx] = g.fillOne(item.info)
			}
		}()
	}
enqueueLoop:
	for idx, info := range infos {
		if ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
			break enqueueLoop
		case jobs <- job{idx: idx, info: info}:
		}
	}
	close(jobs)
	wg.Wait()
	if ctx.Err() != nil {
		var partial []IPInfo
		for _, info := range out {
			if info.IP != "" {
				partial = append(partial, info)
			}
		}
		return partial
	}
	return out
}

func (g *geoService) fillOne(info IPInfo) IPInfo {
	addr, err := netip.ParseAddr(strings.Trim(info.IP, "[]"))
	if err != nil {
		return info
	}
	if g.cityReader != nil {
		record, err := g.cityReader.City(addr)
		if err == nil && record.HasData() {
			country := replaceBlank("None")
			city := replaceBlank("None")
			if record.Country.Names.English != "" {
				country = replaceBlank(record.Country.Names.English)
			}
			if record.City.Names.English != "" {
				city = replaceBlank(record.City.Names.English)
			}
			info.CountryCity = country + "-" + city
		}
	}
	if g.asnReader != nil {
		record, err := g.asnReader.ASN(addr)
		if err == nil && record.HasData() {
			if record.AutonomousSystemOrganization != "" {
				info.Org = record.AutonomousSystemOrganization
			}
			info.ASN = uint(record.AutonomousSystemNumber)
			if record.Network.IsValid() {
				info.Network = record.Network.String()
			}
		}
	}
	return info
}

func replaceBlank(s string) string {
	return strings.ReplaceAll(s, " ", "_")
}

func filterByLocs(in []IPInfo, prefer []string) []IPInfo {
	if len(prefer) == 0 {
		return in
	}
	return filterByLocsWithGeo(in, prefer, nil)
}

func filterByLocsWithGeo(in []IPInfo, prefer []string, geoSvc *geoService) []IPInfo {
	if len(prefer) == 0 {
		return in
	}
	if geoSvc != nil && !geoSvc.locAvailable {
		consolePrint(fmt.Sprintf("%s 数据库不可用, 跳过地区过滤... ...", geoCityDBName))
		return in
	}
	preferNorm := normalizeMatchers(prefer)
	var out []IPInfo
	for _, info := range in {
		countryCityNorm := normalizeCompact(info.CountryCity)
		for _, loc := range preferNorm {
			if strings.Contains(countryCityNorm, loc) {
				out = append(out, info)
				break
			}
		}
	}
	return out
}

func filterByPreferOrgs(in []IPInfo, prefer []string) []IPInfo {
	if len(prefer) == 0 {
		return in
	}
	return filterByPreferOrgsWithGeo(in, prefer, nil)
}

func filterByPreferOrgsWithGeo(in []IPInfo, prefer []string, geoSvc *geoService) []IPInfo {
	if len(prefer) == 0 {
		return in
	}
	if geoSvc != nil && !geoSvc.asnAvailable {
		consolePrint(fmt.Sprintf("%s 数据库不可用, 跳过org 过滤... ...", geoASNDBName))
		return in
	}
	preferNorm := normalizeMatchers(prefer)
	var out []IPInfo
	for _, info := range in {
		orgNorm := normalizeCompact(info.Org)
		for _, org := range preferNorm {
			if strings.Contains(orgNorm, org) {
				out = append(out, info)
				break
			}
		}
	}
	return out
}

func filterByBlockOrgs(in []IPInfo, block []string) []IPInfo {
	if len(block) == 0 {
		return in
	}
	return filterByBlockOrgsWithGeo(in, block, nil)
}

func filterByBlockOrgsWithGeo(in []IPInfo, block []string, geoSvc *geoService) []IPInfo {
	if len(block) == 0 {
		return in
	}
	if geoSvc != nil && !geoSvc.asnAvailable {
		consolePrint(fmt.Sprintf("%s 数据库不可用, 跳过org过滤... ...", geoASNDBName))
		return in
	}
	blockNorm := normalizeMatchers(block)
	var out []IPInfo
	for _, info := range in {
		orgNorm := normalizeCompact(info.Org)
		keep := true
		for _, org := range blockNorm {
			if strings.Contains(orgNorm, org) {
				keep = false
				break
			}
		}
		if keep {
			out = append(out, info)
		}
	}
	return out
}

func normalizeCompact(s string) string {
	s = strings.ToUpper(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

func normalizeMatchers(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, normalizeCompact(item))
	}
	return out
}

func loadVersionFile(path string) map[string]any {
	var res map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}
	}
	_ = json.Unmarshal(data, &res)
	if res == nil {
		return map[string]any{}
	}
	return res
}

func saveVersionFile(path string, data map[string]any) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
