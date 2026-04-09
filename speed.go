package ipcheck

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

func runSpeedTest(ctx context.Context, infos []IPInfo, cfg Config, ctrl *signalController) []IPInfo {
	if !cfg.Speed.Enabled {
		consolePrint("跳过速度测试")
		return infos
	}
	if ctrl != nil {
		ctrl.clearCache()
	}
	consolePrint("准备测试下载速度 ... ...")
	consolePrint(fmt.Sprintf("是否使用user-agent: %v", cfg.Speed.UserAgent))
	if len(infos) > cfg.Speed.IPLimitCount {
		consolePrint(fmt.Sprintf("待测试ip 过多, 当前最大限制数量为%d 个, 压缩中... ...", cfg.Speed.IPLimitCount))
		infos = sampleIPInfos(infos, cfg.Speed.IPLimitCount)
	}
	consolePrint(fmt.Sprintf("正在测试ip 下载速度, 总数为%d", len(infos)))
	var passed []IPInfo
	for idx, info := range infos {
		if ctx.Err() != nil {
			break
		}
		consolePrint(fmt.Sprintf("正在测速第%d/%d个ip: %s:%d %s_%s rtt %.2f ms", idx+1, len(infos), info.ipString(), info.Port, info.Loc, info.Colo, info.RTT))
		fixed := speedSingle(ctx, info, cfg)
		if ctrl != nil {
			ctrl.cache(fixed)
		}
		consolePrint(fixed.infoString())
		if fixed.MaxSpeed >= cfg.Speed.DownloadSpeed && fixed.AvgSpeed >= cfg.Speed.AvgDownloadSpeed {
			passed = append(passed, fixed)
			if cfg.Speed.BetterIPLimit > 0 && len(passed) >= cfg.Speed.BetterIPLimit {
				break
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	return passed
}

func speedSingle(ctx context.Context, info IPInfo, cfg Config) IPInfo {
	timeout := durationSeconds(cfg.Speed.Timeout)
	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	ua := chooseUserAgent(cfg.Speed.UserAgent)

	resp, err := retryRequest(reqCtx, cfg.Speed.MaxRetry, cfg.Speed.RetryFactor, func() (*http.Response, error) {
		return doPinnedGET(reqCtx, info.IP, info.Port, cfg.Speed.URL, "", timeout, ua)
	})
	if err != nil {
		if cfg.Speed.PrintErr {
			consolePrint(fmt.Sprintf("speed test for %s encounters error %v", info.simpleInfo(), err))
		}
		if cfg.Speed.RemoveErrIP {
			info.MaxSpeed = 0
			info.AvgSpeed = 0
		}
		return info
	}

	var (
		size       atomic.Int64
		readErr    atomic.Bool
		startedAt  atomic.Int64
		stopSignal atomic.Bool
	)
	readerDone := make(chan struct{})
	defer func() {
		stopSignal.Store(true)
		_ = resp.Body.Close()
		select {
		case <-readerDone:
		case <-time.After(timeout):
		}
	}()

	go func() {
		defer close(readerDone)
		buf := make([]byte, 16*1024)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				if startedAt.Load() == 0 {
					startedAt.Store(time.Now().UnixNano())
				}
				size.Add(int64(n))
			}
			if stopSignal.Load() {
				return
			}
			if err != nil {
				if err != io.EOF {
					readErr.Store(true)
				}
				return
			}
		}
	}()

	originalStart := time.Now()
	start := originalStart
	var oldSize int64
	for {
		if ctx.Err() != nil {
			stopSignal.Store(true)
			return info
		}
		time.Sleep(100 * time.Millisecond)
		end := time.Now()
		if end.Sub(start) >= 900*time.Millisecond {
			curSize := size.Load()
			if curSize == 0 {
				if end.Sub(originalStart) > durationSeconds(cfg.Speed.DownloadTime*0.5) {
					break
				}
				start = end
				continue
			}
			realStartUnix := startedAt.Load()
			if realStartUnix == 0 {
				continue
			}
			realStart := time.Unix(0, realStartUnix)
			if end.Sub(realStart) < 100*time.Millisecond {
				continue
			}
			freezeEnd := time.Now()
			freezeSize := size.Load()
			speedNow := int(float64(freezeSize-oldSize) / freezeEnd.Sub(start).Seconds() / 1024)
			avgSpeed := speedNow
			if freezeEnd.Sub(realStart) > 900*time.Millisecond {
				avgSpeed = int(float64(freezeSize) / freezeEnd.Sub(realStart).Seconds() / 1024)
			}
			consoleRefresh("  当前下载速度(cur/avg)为: %d/%d kB/s", speedNow, avgSpeed)
			if speedNow > info.MaxSpeed {
				info.MaxSpeed = speedNow
			}
			info.AvgSpeed = avgSpeed
			start = freezeEnd
			oldSize = freezeSize

			if cfg.Speed.FastCheck && freezeEnd.Sub(realStart) > durationSeconds(cfg.Speed.DownloadTime*0.5) {
				if info.MaxSpeed < cfg.Speed.DownloadSpeed/2 || info.AvgSpeed < int(float64(cfg.Speed.AvgDownloadSpeed)*0.77) {
					break
				}
			}
			if freezeEnd.Sub(realStart) > durationSeconds(cfg.Speed.DownloadTime) {
				break
			}
		}
		if readErr.Load() {
			break
		}
	}
	if cfg.Speed.RemoveErrIP && readErr.Load() {
		info.MaxSpeed = 0
		info.AvgSpeed = 0
	}
	return info
}
