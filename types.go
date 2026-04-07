package ipcheck

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	IPPort          int
	NoSave          bool
	CIDRSampleIPNum int
	PureMode        bool

	Runtime struct {
		IPSources   []string
		OutputFile  string
		Verbose     bool
		WhiteList   []string
		BlockList   []string
		PreferLocs  []string
		PreferPorts []int
		PreferOrgs  []string
		BlockOrgs   []string
		DryRun      bool
		OnlyV4      bool
		OnlyV6      bool
	}

	Valid struct {
		Enabled      bool
		IPLimitCount int
		HostName     string
		UserAgent    bool
		Path         string
		FileCheck    bool
		FileURL      string
		ThreadNum    int
		MaxRetry     int
		RetryFactor  float64
		Timeout      float64
		CheckKey     string
		PrintErr     bool
	}

	RTT struct {
		Enabled      bool
		IPLimitCount int
		Interval     float64
		ThreadNum    int
		Timeout      float64
		MaxRTT       float64
		TestCount    int
		MaxLoss      float64
		FastCheck    bool
		PrintErr     bool
	}

	Speed struct {
		Enabled          bool
		IPLimitCount     int
		URL              string
		UserAgent        bool
		MaxRetry         int
		RetryFactor      float64
		Timeout          float64
		DownloadTime     float64
		DownloadSpeed    int
		AvgDownloadSpeed int
		FastCheck        bool
		BetterIPLimit    int
		RemoveErrIP      bool
		PrintErr         bool
	}
}

type GeoConfig struct {
	Proxy    string
	DBAPIURL string
	DBASNURL string
	DBCityURL string
}

type IPInfo struct {
	IP          string
	Port        int
	RTT         float64
	Loss        float64
	MaxSpeed    int
	AvgSpeed    int
	Loc         string
	Colo        string
	CountryCity string
	Org         string
	ASN         uint
	Network     string
	Hostname    string
	STTestTag   string
}

func newIPInfo(ip string, port int) IPInfo {
	return IPInfo{
		IP:          ip,
		Port:        port,
		RTT:         -1,
		Loss:        0,
		MaxSpeed:    -1,
		AvgSpeed:    -1,
		Loc:         "None",
		Colo:        "None",
		CountryCity: "NG-NG",
		Org:         "NG",
		Network:     "None",
	}
}

func (i IPInfo) ipString() string {
	ipStr := i.IP
	if strings.Contains(ipStr, ":") && !strings.HasPrefix(ipStr, "[") {
		ipStr = "[" + ipStr + "]"
	}
	if i.Hostname != "" {
		ipStr = fmt.Sprintf("%s(%s)", i.Hostname, ipStr)
	}
	return ipStr
}

func (i IPInfo) simpleInfo() string {
	return fmt.Sprintf("(%s:%d)", i.ipString(), i.Port)
}

func (i IPInfo) geoInfo() string {
	return fmt.Sprintf("%s(%s)", i.CountryCity, i.Org)
}

func (i IPInfo) geoInfoString() string {
	return fmt.Sprintf("%s 归属: %s ASN: %d CIDR: %s", i.ipString(), i.geoInfo(), i.ASN, i.Network)
}

func (i IPInfo) rttInfo() string {
	return fmt.Sprintf("%s:%d %s_%s %s loss: %s%% rtt: %s ms",
		i.ipString(), i.Port, i.Loc, i.Colo, i.geoInfo(), pyFloat(i.Loss), pyFloat(i.RTT))
}

func (i IPInfo) infoString() string {
	return "  " + i.fileInfoString()
}

func (i IPInfo) fileInfoString() string {
	return fmt.Sprintf("%s:%d%s %s_%s %s loss: %s%% rtt: %s ms, 下载速度(max/avg)为: %d/%d kB/s",
		i.ipString(), i.Port, i.STTestTag, i.Loc, i.Colo, i.geoInfo(), pyFloat(i.Loss), pyFloat(i.RTT), i.MaxSpeed, i.AvgSpeed)
}

type geoReaders struct {
	city *geoDBReader
	asn  *geoDBReader
}

type geoDBReader struct {
	path string
	open bool
}

type httpResult struct {
	Status     int
	Header     map[string][]string
	Body       []byte
	RemoteAddr net.Addr
}

func durationSeconds(v float64) time.Duration {
	return time.Duration(v * float64(time.Second))
}

func pyFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
