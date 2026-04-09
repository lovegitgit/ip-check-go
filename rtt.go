package ipcheck

import (
	"context"
	"fmt"
	"math"
	"net"
	"sort"
	"sync"
	"time"
)

func runRTTTest(ctx context.Context, infos []IPInfo, cfg Config, ctrl *signalController) []IPInfo {
	if !cfg.RTT.Enabled {
		consolePrint("跳过RTT测试")
		return infos
	}
	if ctrl != nil {
		ctrl.clearCache()
	}
	consolePrint("准备测试rtt ... ...")
	consolePrint(fmt.Sprintf("rtt ping 间隔为: %v秒", cfg.RTT.Interval))
	if len(infos) > cfg.RTT.IPLimitCount {
		consolePrint(fmt.Sprintf("待测试ip 过多, 当前最大限制数量为%d 个, 压缩中... ...", cfg.RTT.IPLimitCount))
		infos = sampleIPInfos(infos, cfg.RTT.IPLimitCount)
	}
	consolePrint(fmt.Sprintf("正在测试ip rtt, 总数为%d", len(infos)))

	type result struct{ info IPInfo }
	jobs := make(chan IPInfo)
	results := make(chan result, max(1, cfg.RTT.ThreadNum*2))
	var wg sync.WaitGroup
	for i := 0; i < max(1, cfg.RTT.ThreadNum); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for info := range jobs {
				select {
				case <-ctx.Done():
					return
				case results <- result{info: tcpPing(ctx, info, cfg)}:
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
		if res.info.RTT >= 0 {
			consolePrint(res.info.rttInfo())
			if res.info.RTT <= cfg.RTT.MaxRTT && res.info.Loss <= cfg.RTT.MaxLoss {
				passed = append(passed, res.info)
				passCount++
				if ctrl != nil {
					ctrl.cache(res.info)
				}
			}
		}
		consoleRefresh("  当前进度为: %d/%d, %d pass", testCount, len(infos), passCount)
	}
	sort.Slice(passed, func(i, j int) bool { return passed[i].RTT < passed[j].RTT })
	consolePrint(fmt.Sprintf("rtt 结果为: 总数%d, %d pass", len(infos), len(passed)))
	return passed
}

func tcpPing(ctx context.Context, info IPInfo, cfg Config) IPInfo {
	var (
		sumRTT      float64
		packetsSent int
		packetsLost int
	)
	addr := net.JoinHostPort(info.IP, fmt.Sprint(info.Port))
	timeout := durationSeconds(maxFloat(cfg.RTT.Timeout, cfg.RTT.MaxRTT/800))
	interval := durationSeconds(cfg.RTT.Interval)
	var timer *time.Timer
	if interval > 0 {
		timer = time.NewTimer(interval)
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		defer func() {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}()
	}
	for i := 0; i < cfg.RTT.TestCount; i++ {
		if i > 0 {
			if interval > 0 {
				timer.Reset(interval)
				select {
				case <-ctx.Done():
					return info
				case <-timer.C:
				}
			} else {
				select {
				case <-ctx.Done():
					return info
				default:
				}
			}
		}
		start := time.Now()
		conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", addr)
		if err != nil {
			packetsLost++
			if cfg.RTT.PrintErr {
				consolePrint(fmt.Sprintf("rtt test for %s returns %v", info.simpleInfo(), err))
			}
		} else {
			_ = conn.Close()
			elapsed := float64(time.Since(start)) / float64(time.Millisecond)
			sumRTT += elapsed
			packetsSent++
		}
		if cfg.RTT.FastCheck && packetsSent > 0 {
			loss := float64(packetsLost) * 100 / float64(cfg.RTT.TestCount)
			avg := sumRTT / float64(packetsSent)
			if loss > cfg.RTT.MaxLoss || avg > cfg.RTT.MaxRTT {
				return info
			}
		}
	}
	if packetsSent == 0 {
		return info
	}
	info.RTT = round2(sumRTT / float64(packetsSent))
	info.Loss = round2(float64(packetsLost) * 100 / float64(cfg.RTT.TestCount))
	return info
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
