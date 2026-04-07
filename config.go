package ipcheck

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
)

func defaultConfig() Config {
	cfg := Config{
		IPPort:          443,
		CIDRSampleIPNum: 16,
	}
	cfg.Valid.Enabled = true
	cfg.Valid.IPLimitCount = 100000
	cfg.Valid.HostName = "cloudflare.com"
	cfg.Valid.Path = "/cdn-cgi/trace"
	cfg.Valid.FileCheck = true
	cfg.Valid.FileURL = "https://speed.cloudflare.com/__down?bytes=500000000"
	cfg.Valid.ThreadNum = 64
	cfg.Valid.MaxRetry = 2
	cfg.Valid.RetryFactor = 0.5
	cfg.Valid.Timeout = 1.5
	cfg.Valid.CheckKey = "h"

	cfg.RTT.Enabled = true
	cfg.RTT.IPLimitCount = 100000
	cfg.RTT.Interval = 0.5
	cfg.RTT.ThreadNum = 32
	cfg.RTT.Timeout = 0.5
	cfg.RTT.MaxRTT = 300
	cfg.RTT.TestCount = 10
	cfg.RTT.MaxLoss = 100

	cfg.Speed.Enabled = true
	cfg.Speed.IPLimitCount = 100000
	cfg.Speed.URL = "https://speed.cloudflare.com/__down?bytes=500000000"
	cfg.Speed.MaxRetry = 10
	cfg.Speed.RetryFactor = 0.5
	cfg.Speed.Timeout = 5
	cfg.Speed.DownloadTime = 10
	cfg.Speed.DownloadSpeed = 5000
	cfg.Speed.BetterIPLimit = 0
	return cfg
}

func loadConfig(path string) (Config, error) {
	cfg := defaultConfig()
	sections, err := parseINIFile(path)
	if err != nil {
		return cfg, err
	}
	common := sections["common"]
	cfg.IPPort = intValue(common, "ip_port", cfg.IPPort)
	cfg.NoSave = boolValue(common, "no_save", cfg.NoSave)
	cfg.CIDRSampleIPNum = intValue(common, "cidr_sample_ip_num", cfg.CIDRSampleIPNum)

	valid := sections["valid test"]
	cfg.Valid.Enabled = boolValue(valid, "enabled", cfg.Valid.Enabled)
	cfg.Valid.IPLimitCount = intValue(valid, "ip_limit_count", cfg.Valid.IPLimitCount)
	cfg.Valid.HostName = stringValue(valid, "host_name", cfg.Valid.HostName)
	cfg.Valid.UserAgent = boolValue(valid, "user_agent", cfg.Valid.UserAgent)
	cfg.Valid.Path = stringValue(valid, "path", cfg.Valid.Path)
	cfg.Valid.FileCheck = boolValue(valid, "file_check", cfg.Valid.FileCheck)
	cfg.Valid.FileURL = stringValue(valid, "file_url", cfg.Valid.FileURL)
	cfg.Valid.ThreadNum = intValue(valid, "thread_num", cfg.Valid.ThreadNum)
	cfg.Valid.MaxRetry = intValue(valid, "max_retry", cfg.Valid.MaxRetry)
	cfg.Valid.RetryFactor = floatValue(valid, "retry_factor", cfg.Valid.RetryFactor)
	cfg.Valid.Timeout = floatValue(valid, "timeout", cfg.Valid.Timeout)
	cfg.Valid.PrintErr = boolValue(valid, "print_err", cfg.Valid.PrintErr)

	rtt := sections["rtt test"]
	cfg.RTT.Enabled = boolValue(rtt, "enabled", cfg.RTT.Enabled)
	cfg.RTT.IPLimitCount = intValue(rtt, "ip_limit_count", cfg.RTT.IPLimitCount)
	cfg.RTT.Interval = floatValue(rtt, "interval", cfg.RTT.Interval)
	cfg.RTT.ThreadNum = intValue(rtt, "thread_num", cfg.RTT.ThreadNum)
	cfg.RTT.Timeout = floatValue(rtt, "timeout", cfg.RTT.Timeout)
	cfg.RTT.MaxRTT = floatValue(rtt, "max_rtt", cfg.RTT.MaxRTT)
	cfg.RTT.TestCount = intValue(rtt, "test_count", cfg.RTT.TestCount)
	cfg.RTT.MaxLoss = floatValue(rtt, "max_loss", cfg.RTT.MaxLoss)
	cfg.RTT.FastCheck = boolValue(rtt, "fast_check", cfg.RTT.FastCheck)
	cfg.RTT.PrintErr = boolValue(rtt, "print_err", cfg.RTT.PrintErr)

	speed := sections["speed test"]
	cfg.Speed.Enabled = boolValue(speed, "enabled", cfg.Speed.Enabled)
	cfg.Speed.IPLimitCount = intValue(speed, "ip_limit_count", cfg.Speed.IPLimitCount)
	cfg.Speed.URL = stringValue(speed, "url", cfg.Speed.URL)
	cfg.Speed.UserAgent = boolValue(speed, "user_agent", cfg.Speed.UserAgent)
	cfg.Speed.MaxRetry = intValue(speed, "max_retry", cfg.Speed.MaxRetry)
	cfg.Speed.RetryFactor = floatValue(speed, "retry_factor", cfg.Speed.RetryFactor)
	cfg.Speed.Timeout = floatValue(speed, "timeout", cfg.Speed.Timeout)
	cfg.Speed.DownloadTime = floatValue(speed, "download_time", cfg.Speed.DownloadTime)
	cfg.Speed.DownloadSpeed = intValue(speed, "download_speed", cfg.Speed.DownloadSpeed)
	cfg.Speed.AvgDownloadSpeed = intValue(speed, "avg_download_speed", cfg.Speed.AvgDownloadSpeed)
	cfg.Speed.FastCheck = boolValue(speed, "fast_check", cfg.Speed.FastCheck)
	cfg.Speed.BetterIPLimit = intValue(speed, "bt_ip_limit", cfg.Speed.BetterIPLimit)
	cfg.Speed.RemoveErrIP = boolValue(speed, "rm_err_ip", cfg.Speed.RemoveErrIP)
	cfg.Speed.PrintErr = boolValue(speed, "print_err", cfg.Speed.PrintErr)
	return cfg, nil
}

func defaultGeoConfigValue() GeoConfig {
	return GeoConfig{
		DBAPIURL: "https://api.github.com/repos/P3TERX/GeoLite.mmdb/releases/latest",
		DBASNURL: "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-ASN.mmdb",
		DBCityURL: "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-City.mmdb",
	}
}

func loadGeoConfig(path string) (GeoConfig, error) {
	cfg := defaultGeoConfigValue()
	sections, err := parseINIFile(path)
	if err != nil {
		return cfg, err
	}
	common := sections["common"]
	cfg.Proxy = stringValue(common, "proxy", cfg.Proxy)
	cfg.DBAPIURL = stringValue(common, "db_api_url", cfg.DBAPIURL)
	cfg.DBASNURL = stringValue(common, "db_asn_url", cfg.DBASNURL)
	cfg.DBCityURL = stringValue(common, "db_city_url", cfg.DBCityURL)
	return cfg, nil
}

func stringValue(section map[string]string, key, def string) string {
	if section == nil {
		return def
	}
	if val, ok := section[key]; ok {
		return val
	}
	return def
}

func boolValue(section map[string]string, key string, def bool) bool {
	val := stringValue(section, key, "")
	if val == "" {
		return def
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return def
	}
	return parsed
}

func intValue(section map[string]string, key string, def int) int {
	val := stringValue(section, key, "")
	if val == "" {
		return def
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return parsed
}

func floatValue(section map[string]string, key string, def float64) float64 {
	val := stringValue(section, key, "")
	if val == "" {
		return def
	}
	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return def
	}
	return parsed
}

func ensureIPCheckConfig(paths appPaths, path string) (string, error) {
	if err := ensureExampleConfigs(paths); err != nil {
		return "", err
	}
	if path == "" {
		path = paths.ipCheckConfig
	}
	template, err := readTemplateFile(paths.ipCheckConfigEx, defaultIPCheckConfig)
	if err != nil {
		return "", err
	}
	created, err := writeIfMissing(path, template)
	if err != nil {
		return "", err
	}
	if created {
		consolePrint("警告: 配置文件不存在, 正在生成默认配置... ...")
		consolePrint(fmt.Sprintf("配置文件已生成位于 %s", path))
	}
	return path, nil
}

func ensureGeoConfig(paths appPaths) (string, error) {
	if err := ensureExampleConfigs(paths); err != nil {
		return "", err
	}
	template, err := readTemplateFile(paths.geoConfigEx, defaultGeoConfig)
	if err != nil {
		return "", err
	}
	created, err := writeIfMissing(paths.geoConfig, template)
	if err != nil {
		return "", err
	}
	if created {
		consolePrint("警告: 配置文件不存在, 正在生成默认配置... ...")
		consolePrint(fmt.Sprintf("配置文件已生成位于 %s, 请按需要修改!", paths.geoConfig))
	}
	return paths.geoConfig, nil
}

func ensureExampleConfigs(paths appPaths) error {
	if _, err := writeIfMissing(paths.ipCheckConfigEx, defaultIPCheckConfig); err != nil {
		return err
	}
	if _, err := writeIfMissing(paths.geoConfigEx, defaultGeoConfig); err != nil {
		return err
	}
	return nil
}

func readTemplateFile(path, fallback string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return "", err
	}
	return string(data), nil
}

func parseURLParts(raw string) (host, path string, err error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}
	if parsed.Scheme == "" || parsed.Hostname() == "" {
		return "", "", fmt.Errorf("invalid url: %s", raw)
	}
	path = parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}
	return parsed.Hostname(), path, nil
}

func openEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad.exe"
		} else {
			editor = "vi"
		}
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
