package ipcheck

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
)

func runValidTest(ctx context.Context, infos []IPInfo, cfg Config, ctrl *signalController) []IPInfo {
	if !cfg.Valid.Enabled {
		consolePrint("跳过可用性测试")
		return infos
	}
	if ctrl != nil {
		ctrl.clearCache()
	}
	consolePrint("准备测试可用性 ... ...")
	if len(infos) > cfg.Valid.IPLimitCount {
		consolePrint(fmt.Sprintf("待测试ip 过多, 当前最大限制数量为%d 个, 压缩中... ...", cfg.Valid.IPLimitCount))
		infos = sampleIPInfos(infos, cfg.Valid.IPLimitCount)
	}
	consolePrint(fmt.Sprintf("可用性域名为: %s", cfg.Valid.HostName))
	consolePrint(fmt.Sprintf("是否使用user-agent: %v", cfg.Valid.UserAgent))
	consolePrint(fmt.Sprintf("正在测试ip可用性, 总数为%d", len(infos)))

	type result struct {
		info IPInfo
		ok   bool
	}
	jobs := make(chan IPInfo)
	results := make(chan result, max(1, cfg.Valid.ThreadNum*2))
	var wg sync.WaitGroup
	workerCount := max(1, cfg.Valid.ThreadNum)
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for info := range jobs {
				fixed, ok := validSingle(ctx, info, cfg)
				select {
				case <-ctx.Done():
					return
				case results <- result{info: fixed, ok: ok}:
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, info := range infos {
			select {
			case <-ctx.Done():
				return
			case jobs <- info:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var passed []IPInfo
	testCount := 0
	passCount := 0
	for res := range results {
		testCount++
		if res.ok {
			passCount++
			passed = append(passed, res.info)
			if ctrl != nil {
				ctrl.cache(res.info)
			}
		}
		consoleRefresh("  当前进度为: %d/%d, %d pass", testCount, len(infos), passCount)
	}
	consolePrint(fmt.Sprintf("可用性结果为: 总数%d, %d pass", len(infos), len(passed)))
	return passed
}

func validSingle(ctx context.Context, info IPInfo, cfg Config) (IPInfo, bool) {
	traceURL := fmt.Sprintf("https://%s%s", cfg.Valid.HostName, cfg.Valid.Path)
	ua := chooseUserAgent(cfg.Valid.UserAgent)
	resp, err := retryRequest(ctx, cfg.Valid.MaxRetry, cfg.Valid.RetryFactor, func() (*http.Response, error) {
		return doPinnedGET(ctx, info.IP, info.Port, traceURL, cfg.Valid.HostName, durationSeconds(cfg.Valid.Timeout), ua)
	})
	if err != nil {
		if cfg.Valid.PrintErr {
			consolePrint(fmt.Sprintf("valid test for %s encounters err %v", info.simpleInfo(), err))
		}
		return info, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info, false
	}
	body, err := readLimitedBody(resp.Body, 64*1024)
	if err != nil {
		return info, false
	}
	if ctx.Err() != nil {
		return info, false
	}
	if cfg.Valid.Path == "/cdn-cgi/trace" {
		ok, loc, colo := parseTraceBody(body, cfg.Valid.CheckKey, cfg.Valid.HostName)
		if !ok {
			return info, false
		}
		info.Loc = loc
		info.Colo = colo
	}

	if cfg.Valid.FileCheck && cfg.Valid.FileURL != "" {
		if ctx.Err() != nil {
			return info, false
		}
		fileHost, _, err := parseURLParts(cfg.Valid.FileURL)
		if err != nil {
			return info, false
		}
		fileUA := chooseUserAgent(cfg.Valid.UserAgent)
		fileResp, err := retryRequest(ctx, cfg.Valid.MaxRetry, cfg.Valid.RetryFactor, func() (*http.Response, error) {
			return doPinnedGET(ctx, info.IP, info.Port, cfg.Valid.FileURL, fileHost, durationSeconds(cfg.Valid.Timeout), fileUA)
		})
		if err != nil {
			if cfg.Valid.PrintErr {
				consolePrint(fmt.Sprintf("valid file test for %s encounters err %v", info.simpleInfo(), err))
			}
			return info, false
		}
		defer fileResp.Body.Close()
		if fileResp.StatusCode != http.StatusOK {
			if cfg.Valid.PrintErr {
				consolePrint(fmt.Sprintf("valid file_url test for %s got status %d from %s", info.simpleInfo(), fileResp.StatusCode, cfg.Valid.FileURL))
			}
			return info, false
		}
		if fileResp.Header.Get("Content-Length") == "" {
			if cfg.Valid.PrintErr {
				consolePrint(fmt.Sprintf("valid file_url test for %s got invalid Content-Length: %q from %s", info.simpleInfo(), fileResp.Header.Get("Content-Length"), cfg.Valid.FileURL))
			}
			return info, false
		}
	}
	return info, true
}

func parseTraceBody(body []byte, checkKey, hostName string) (ok bool, loc string, colo string) {
	checkKeyBytes := []byte(checkKey)
	hostNameBytes := []byte(hostName)
	locKey := []byte("loc")
	coloKey := []byte("colo")
	for len(body) > 0 {
		line := body
		if idx := bytes.IndexByte(body, '\n'); idx >= 0 {
			line = body[:idx]
			body = body[idx+1:]
		} else {
			body = nil
		}
		line = bytes.TrimSpace(bytes.TrimSuffix(line, []byte{'\r'}))
		if len(line) == 0 {
			continue
		}
		key, value, found := bytes.Cut(line, []byte("="))
		if !found {
			continue
		}
		switch {
		case bytes.Equal(key, checkKeyBytes):
			if !bytes.Equal(value, hostNameBytes) {
				return false, "", ""
			}
			ok = true
		case bytes.Equal(key, locKey):
			loc = string(value)
		case bytes.Equal(key, coloKey):
			colo = string(value)
		}
	}
	return ok, loc, colo
}
