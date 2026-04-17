package ipcheck

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ErrUsage = errors.New("usage")

func RunIPCheck(ctx context.Context, args []string) error {
	paths := newAppPaths()
	sigCtrl := newSignalController()
	stopSignals := installSignalHandler(sigCtrl)
	defer stopSignals()
	fs := flag.NewFlagSet("ip-check", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "usage: ip-check [options] source [source ...]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "ip-check 参数")
		fmt.Fprintln(os.Stdout)
		fs.PrintDefaults()
	}

	var (
		whiteList  stringList
		blockList  stringList
		preferLocs stringList
		preferOrgs stringList
		blockOrgs  stringList
		preferPorts intList
		maxVT int
		maxRT int
		maxST int
		maxBT int
		port  int
		host  string
		disableRT bool
		disableVT bool
		disableST bool
		output string
		fastCheck bool
		speed int
		avgSpeed int
		rtt int
		loss int
		configPath string
		testURL string
		verbose bool
		noSave bool
		dryRun bool
		onlyV4 bool
		onlyV6 bool
		crSize int
		resolveThreadNum int
		disableFileCheck bool
		pureMode bool
		showVersion bool
	)
	fs.Var(&whiteList, "w", "偏好ip参数, 可重复传入, 如 -w 8 -w 9")
	fs.Var(&whiteList, "white_list", "偏好ip参数, 可重复传入, 如 -white_list 8 -white_list 9")
	fs.Var(&blockList, "b", "屏蔽ip参数, 可重复传入")
	fs.Var(&blockList, "block_list", "屏蔽ip参数, 可重复传入")
	fs.Var(&preferLocs, "pl", "偏好国家地区, 可重复传入, 如 -pl hongkong -pl japan")
	fs.Var(&preferLocs, "prefer_locs", "偏好国家地区, 可重复传入")
	fs.Var(&preferOrgs, "po", "偏好org, 可重复传入")
	fs.Var(&preferOrgs, "prefer_orgs", "偏好org, 可重复传入")
	fs.Var(&blockOrgs, "bo", "屏蔽org, 可重复传入")
	fs.Var(&blockOrgs, "block_orgs", "屏蔽org, 可重复传入")
	fs.Var(&preferPorts, "pp", "针对 ip:port 测试源筛选端口, 可重复传入")
	fs.Var(&preferPorts, "prefer_ports", "针对 ip:port 测试源筛选端口, 可重复传入")
	fs.IntVar(&maxVT, "lv", 0, "最大用来检测有效(valid) ip数量限制")
	fs.IntVar(&maxVT, "max_vt_ip_count", 0, "最大用来检测有效(valid) ip数量限制")
	fs.IntVar(&maxRT, "lr", 0, "最大用来检测rtt ip数量限制")
	fs.IntVar(&maxRT, "max_rt_ip_count", 0, "最大用来检测rtt ip数量限制")
	fs.IntVar(&maxST, "ls", 0, "最大用来检测下载(speed)速度的ip数量限制")
	fs.IntVar(&maxST, "max_st_ip_count", 0, "最大用来检测下载(speed)速度的ip数量限制")
	fs.IntVar(&maxBT, "lb", 0, "最大better ip的ip数量限制")
	fs.IntVar(&maxBT, "max_bt_ip_count", 0, "最大better ip的ip数量限制")
	fs.IntVar(&port, "p", 443, "用来检测的端口")
	fs.IntVar(&port, "port", 443, "用来检测的端口")
	fs.StringVar(&host, "H", "", "可用性域名")
	fs.StringVar(&host, "host", "", "可用性域名")
	fs.BoolVar(&disableRT, "dr", false, "是否禁用RTT测试")
	fs.BoolVar(&disableRT, "disable_rt", false, "是否禁用RTT测试")
	fs.BoolVar(&disableVT, "dv", false, "是否禁用可用性测试")
	fs.BoolVar(&disableVT, "disable_vt", false, "是否禁用可用性测试")
	fs.BoolVar(&disableST, "ds", false, "是否禁用速度测试")
	fs.BoolVar(&disableST, "disable_st", false, "是否禁用速度测试")
	fs.StringVar(&output, "o", "", "输出文件")
	fs.StringVar(&output, "output", "", "输出文件")
	fs.BoolVar(&fastCheck, "f", false, "是否执行快速测试")
	fs.BoolVar(&fastCheck, "fast_check", false, "是否执行快速测试")
	fs.IntVar(&speed, "s", 0, "期望ip的最低网速(kB/s)")
	fs.IntVar(&speed, "speed", 0, "期望ip的最低网速(kB/s)")
	fs.IntVar(&avgSpeed, "as", 0, "期望ip的最低平均网速(kB/s)")
	fs.IntVar(&avgSpeed, "avg_speed", 0, "期望ip的最低平均网速(kB/s)")
	fs.IntVar(&rtt, "r", 0, "期望的最大rtt(ms)")
	fs.IntVar(&rtt, "rtt", 0, "期望的最大rtt(ms)")
	fs.IntVar(&loss, "l", -1, "期望的最大丢包率")
	fs.IntVar(&loss, "loss", -1, "期望的最大丢包率")
	fs.StringVar(&configPath, "c", "", "配置文件")
	fs.StringVar(&configPath, "config", "", "配置文件")
	fs.StringVar(&testURL, "u", "", "测速地址")
	fs.StringVar(&testURL, "url", "", "测速地址")
	fs.BoolVar(&verbose, "v", false, "显示调试信息")
	fs.BoolVar(&verbose, "verbose", false, "显示调试信息")
	fs.BoolVar(&noSave, "ns", false, "是否忽略保存测速结果文件")
	fs.BoolVar(&noSave, "no_save", false, "是否忽略保存测速结果文件")
	fs.BoolVar(&dryRun, "dry_run", false, "是否跳过所有测试")
	fs.BoolVar(&onlyV4, "4", false, "仅测试ipv4")
	fs.BoolVar(&onlyV4, "only_v4", false, "仅测试ipv4")
	fs.BoolVar(&onlyV6, "6", false, "仅测试ipv6")
	fs.BoolVar(&onlyV6, "only_v6", false, "仅测试ipv6")
	fs.IntVar(&crSize, "cs", 0, "cidr 随机抽样ip数量限制")
	fs.IntVar(&crSize, "cr_size", 0, "cidr 随机抽样ip数量限制")
	fs.BoolVar(&disableFileCheck, "df", false, "是否禁用可用性检测文件可用性")
	fs.BoolVar(&disableFileCheck, "disable_file_check", false, "是否禁用可用性检测文件可用性")
	fs.BoolVar(&pureMode, "pure_mode", false, "纯净模式, 不使用geo数据库进行ip信息补全")
	fs.IntVar(&resolveThreadNum, "rs", 0, "域名解析线程数")
	fs.IntVar(&resolveThreadNum, "resolve_thread_num", 0, "域名解析线程数")
	fs.BoolVar(&showVersion, "version", false, "显示版本信息")
	normalizedArgs := normalizeArgs(args, ipCheckArgSpec())
	if err := fs.Parse(normalizedArgs); err != nil {
		printIPCheckUsage()
		return ErrUsage
	}
	if showVersion {
		consolePrint(fmt.Sprintf("ip-check version %s installed in %s", version, paths.baseDir))
		return nil
	}
	if fs.NArg() == 0 {
		printIPCheckUsage()
		return ErrUsage
	}

	configPath, err := ensureIPCheckConfig(paths, configPath)
	if err != nil {
		return err
	}
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	cfg.PureMode = pureMode
	cfg.Runtime.Verbose = verbose
	if cfg.Runtime.Verbose {
		cfg.Valid.PrintErr = true
		cfg.RTT.PrintErr = true
		cfg.Speed.PrintErr = true
	}
	cfg.Runtime.IPSources = uniqueStrings(fs.Args())
	cfg.Runtime.WhiteList = uniqueStrings([]string(whiteList))
	if len(blockList) > 0 && len(cfg.Runtime.WhiteList) > 0 {
		consolePrint("偏好参数与黑名单参数同时存在, 自动忽略黑名单参数!")
	}
	if len(cfg.Runtime.WhiteList) == 0 {
		cfg.Runtime.BlockList = uniqueStrings([]string(blockList))
	}
	cfg.Runtime.PreferLocs = uniqueStrings([]string(preferLocs))
	if len(cfg.Runtime.WhiteList) > 0 {
		consolePrint("白名单参数为:", pyStringList(cfg.Runtime.WhiteList))
	}
	if len(cfg.Runtime.BlockList) > 0 {
		consolePrint("黑名单参数为:", pyStringList(cfg.Runtime.BlockList))
	}
	if len(cfg.Runtime.PreferLocs) > 0 {
		consolePrint("优选地区参数为:", pyStringList(cfg.Runtime.PreferLocs))
	}
	cfg.Runtime.PreferOrgs = uniqueStrings([]string(preferOrgs))
	if len(blockOrgs) > 0 && len(cfg.Runtime.PreferOrgs) > 0 {
		consolePrint("偏好org参数与屏蔽org参数同时存在, 自动忽略屏蔽org参数!")
	}
	if len(cfg.Runtime.PreferOrgs) == 0 {
		cfg.Runtime.BlockOrgs = uniqueStrings([]string(blockOrgs))
	}
	if len(cfg.Runtime.PreferOrgs) > 0 {
		consolePrint("优选org 参数为:", pyStringList(cfg.Runtime.PreferOrgs))
	}
	if len(cfg.Runtime.BlockOrgs) > 0 {
		consolePrint("屏蔽org 参数为:", pyStringList(cfg.Runtime.BlockOrgs))
	}
	cfg.Runtime.PreferPorts = []int(preferPorts)
	if len(cfg.Runtime.PreferPorts) > 0 {
		consolePrint("ip:port 测试源端口为:", pyIntList(cfg.Runtime.PreferPorts))
	}
	cfg.Valid.Enabled = !disableVT
	cfg.RTT.Enabled = !disableRT
	cfg.Speed.Enabled = !disableST
	cfg.Runtime.DryRun = dryRun
	cfg.Runtime.OnlyV4 = onlyV4
	cfg.Runtime.OnlyV6 = onlyV6
	cfg.IPPort = port
	cfg.Valid.FileCheck = !disableFileCheck
	cfg.NoSave = noSave
	if host != "" {
		cfg.Valid.HostName = host
	}
	if rtt > 0 {
		cfg.RTT.MaxRTT = float64(rtt)
	}
	if speed > 0 {
		cfg.Speed.DownloadSpeed = speed
	}
	if avgSpeed > 0 {
		cfg.Speed.AvgDownloadSpeed = avgSpeed
	}
	if cfg.Speed.AvgDownloadSpeed > cfg.Speed.DownloadSpeed {
		cfg.Speed.AvgDownloadSpeed = cfg.Speed.DownloadSpeed
	}
	if fastCheck {
		cfg.Speed.FastCheck = true
	}
	if loss >= 0 {
		cfg.RTT.MaxLoss = float64(loss)
	}
	if maxVT > 0 {
		cfg.Valid.IPLimitCount = maxVT
	}
	if maxRT > 0 {
		cfg.RTT.IPLimitCount = maxRT
	}
	if maxST > 0 {
		cfg.Speed.IPLimitCount = maxST
	}
	if maxBT > 0 {
		cfg.Speed.BetterIPLimit = maxBT
	}
	if crSize > 0 {
		cfg.CIDRSampleIPNum = crSize
	}
	if resolveThreadNum > 0 {
		cfg.Runtime.ResolveThreadNum = resolveThreadNum
	}
	if testURL != "" {
		hostName, path, err := parseURLParts(testURL)
		if err != nil {
			return err
		}
		cfg.Valid.HostName = hostName
		cfg.Valid.Path = path
		cfg.Valid.FileURL = testURL
		cfg.Speed.URL = testURL
	}
	if output != "" {
		cfg.Runtime.OutputFile = output
	} else {
		cfg.Runtime.OutputFile = defaultOutputPath(cfg.Runtime.IPSources[0], cfg.IPPort)
	}
	if cfg.Runtime.DryRun {
		consolePrint("跳过所有测试!!!")
		cfg.Valid.Enabled = false
		cfg.RTT.Enabled = false
		cfg.Speed.Enabled = false
	}
	if !cfg.Valid.Enabled {
		consolePrint("可用性检测已关闭")
	}
	if !cfg.RTT.Enabled {
		consolePrint("rtt 测试已关闭")
	}
	if !cfg.Speed.Enabled {
		consolePrint("速度测试已关闭")
	}

	consolePrint(fmt.Sprintf("当前配置文件为: %s", configPath))
	consolePrint(fmt.Sprintf("纯净模式: %s", pyBool(cfg.PureMode)))
	consolePrint(fmt.Sprintf("是否开启调试信息: %s", pyBool(cfg.Runtime.Verbose)))
	consolePrint(fmt.Sprintf("测试源文件为: %s", pyStringList(cfg.Runtime.IPSources)))
	consolePrint(fmt.Sprintf("测试端口为: %d", cfg.IPPort))
	consolePrint(fmt.Sprintf("可用性测试文件检测开关为: %s", pyBool(cfg.Valid.FileCheck)))
	consolePrint(fmt.Sprintf("期望最大rtt 为: %s ms", pyFloat(cfg.RTT.MaxRTT)))
	consolePrint(fmt.Sprintf("期望网速为: %d kB/s", cfg.Speed.DownloadSpeed))
	consolePrint(fmt.Sprintf("期望平均网速为: %d kB/s", cfg.Speed.AvgDownloadSpeed))
	if cfg.Speed.FastCheck {
		consolePrint("快速测速已开启")
	}
	consolePrint(fmt.Sprintf("优选ip 文件为: %s", cfg.Runtime.OutputFile))
	consolePrint(fmt.Sprintf("是否忽略保存测速结果到文件: %s", pyBool(cfg.NoSave || cfg.Runtime.DryRun)))
	consolePrint(fmt.Sprintf("cidr 抽样ip 个数为: %d", cfg.CIDRSampleIPNum))
	consolePrint(fmt.Sprintf("域名解析线程数为: %d", cfg.Runtime.ResolveThreadNum))

	geoSvc, err := openGeoService(paths)
	if err != nil {
		return err
	}
	if geoSvc != nil {
		defer geoSvc.Close()
	}
	infos, metrics, err := parseSources(ctx, cfg, geoSvc, true)
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		return fmt.Errorf("没有从参数中生产待测试ip 列表, 请检查参数")
	}
	consolePrint(fmt.Sprintf("解析ip 耗时: %s秒", pyFloat(metrics.ResolveSeconds)))
	consolePrint(fmt.Sprintf("获取geo 信息耗时: %s秒", pyFloat(metrics.GeoSeconds)))
	consolePrint(fmt.Sprintf("预处理ip 总计耗时: %s秒", pyFloat(metrics.TotalSeconds)))
	consolePrint(fmt.Sprintf("从参数中生成了%d 个待测试ip", len(infos)))
	infos = filterByPreferOrgsWithGeo(infos, cfg.Runtime.PreferOrgs, geoSvc)
	infos = filterByBlockOrgsWithGeo(infos, cfg.Runtime.BlockOrgs, geoSvc)
	infos = filterByLocsWithGeo(infos, cfg.Runtime.PreferLocs, geoSvc)
	shuffleIPInfos(infos)

	if cfg.Runtime.DryRun {
		consolePrint("跳过可用性测试")
		consolePrint("跳过RTT测试")
		consolePrint("跳过速度测试")
		printBetterIPs(infos)
		return nil
	}

	geoCfg, _ := loadGeoConfig(paths.geoConfig)
	updateChan := make(chan string, 1)
	go checkGeoUpdate(ctx, paths, geoCfg, updateChan)
	defer func() {
		select {
		case msg := <-updateChan:
			consolePrint(msg)
		default:
		}
	}()

	validCtx, validCancel := context.WithCancel(ctx)
	sigCtrl.setStage(stageValid, validCancel)
	passed := runValidTest(validCtx, infos, cfg, sigCtrl)
	if validCtx.Err() != nil {
		sigCtrl.printCache()
	}
	sigCtrl.clearStage()
	validCancel()
	if len(passed) == 0 {
		consolePrint("可用性测试没有获取到可用ip, 测试停止!")
		return nil
	}
	rttCtx, rttCancel := context.WithCancel(ctx)
	sigCtrl.setStage(stageRTT, rttCancel)
	passed = runRTTTest(rttCtx, passed, cfg, sigCtrl)
	if rttCtx.Err() != nil {
		sigCtrl.printCache()
	}
	sigCtrl.clearStage()
	rttCancel()
	if len(passed) == 0 {
		consolePrint("rtt 测试没有获取到可用ip, 测试停止!")
		return nil
	}
	speedCtx, speedCancel := context.WithCancel(ctx)
	sigCtrl.setStage(stageSpeed, speedCancel)
	passed = runSpeedTest(speedCtx, passed, cfg, sigCtrl)
	if speedCtx.Err() != nil {
		sigCtrl.printCache()
	}
	sigCtrl.clearStage()
	speedCancel()
	sigCtrl.finish()
	if len(passed) == 0 {
		consolePrint("下载测试没有获取到可用ip, 测试停止!")
		return nil
	}
	sort.Slice(passed, func(i, j int) bool { return passed[i].MaxSpeed > passed[j].MaxSpeed })
	printBetterIPs(passed)
	if !(cfg.NoSave || cfg.Runtime.DryRun) {
		if cfg.PureMode {
			return writePureIPs(passed, cfg.Runtime.OutputFile)
		}
		return writeBetterIPs(passed, cfg.Runtime.OutputFile)
	}
	return nil
}

func RunIPCheckCfg(args []string) error {
	paths := newAppPaths()
	fs := flag.NewFlagSet("ip-check-cfg", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "usage: ip-check-cfg [options]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "ip-check 参数配置向导")
		fmt.Fprintln(os.Stdout)
		fs.PrintDefaults()
	}
	output := fs.String("o", paths.ipCheckConfig, "")
	fs.StringVar(output, "output", paths.ipCheckConfig, "参数配置文件路径")
	example := fs.Bool("e", false, "")
	fs.BoolVar(example, "example", false, "显示配置文件示例")
	if err := fs.Parse(normalizeArgs(args, cfgArgSpec())); err != nil {
		return ErrUsage
	}
	if *example {
		fmt.Print(defaultIPCheckConfig)
		return nil
	}
	path, err := ensureIPCheckConfig(paths, *output)
	if err != nil {
		return err
	}
	consolePrint(fmt.Sprintf("编辑配置文件 %s", path))
	return openEditor(path)
}

func RunGeoInfo(ctx context.Context, args []string) error {
	paths := newAppPaths()
	fs := flag.NewFlagSet("igeo-info", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "usage: igeo-info [options] ip [ip ...]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "geo-info 获取ip(s) 的归属地信息")
	}
	if err := fs.Parse(normalizeArgs(args, geoInfoArgSpec())); err != nil {
		return ErrUsage
	}
	if fs.NArg() == 0 {
		return ErrUsage
	}
	cfg := defaultConfig()
	cfg.Runtime.IPSources = uniqueStrings(fs.Args())
	svc, err := openGeoService(paths)
	if err != nil {
		return err
	}
	defer svc.Close()
	infos, _, err := parseSources(ctx, cfg, svc, true)
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		consolePrint("请检查是否输入了有效ip(s)")
		return nil
	}
	for _, info := range infos {
		consolePrint(info.geoInfoString())
	}
	return nil
}

func RunGeoDownload(ctx context.Context, args []string) error {
	paths := newAppPaths()
	fs := flag.NewFlagSet("igeo-dl", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "usage: igeo-dl [options]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "igeo-dl 升级/下载geo 数据库")
		fmt.Fprintln(os.Stdout)
		fs.PrintDefaults()
	}
	urlArg := fs.String("u", "", "")
	fs.StringVar(urlArg, "url", "", "geo数据库下载地址, 要求结尾包含 GeoLite2-City.mmdb 或 GeoLite2-ASN.mmdb")
	proxyArg := fs.String("p", "", "")
	fs.StringVar(proxyArg, "proxy", "", "下载时使用的代理")
	autoYes := fs.Bool("y", false, "")
	fs.BoolVar(autoYes, "yes", false, "自动确认更新并下载 GEO 数据库")
	if err := fs.Parse(normalizeArgs(args, geoDownloadArgSpec())); err != nil {
		return ErrUsage
	}
	cfgPath, err := ensureGeoConfig(paths)
	if err != nil {
		return err
	}
	geoCfg, err := loadGeoConfig(cfgPath)
	if err != nil {
		return err
	}
	if *proxyArg != "" {
		geoCfg.Proxy = *proxyArg
	}
	if *urlArg != "" {
		switch {
		case hasSuffixAny(*urlArg, geoCityDBName):
			consolePrint("CITY 数据库下载地址:", *urlArg)
			return downloadFile(ctx, *urlArg, paths.geoCityDB, geoCfg.Proxy)
		case hasSuffixAny(*urlArg, geoASNDBName):
			consolePrint("ASN 数据库下载地址:", *urlArg)
			return downloadFile(ctx, *urlArg, paths.geoASNDB, geoCfg.Proxy)
		default:
			return fmt.Errorf("请输入包含%s 或 %s 的url", geoCityDBName, geoASNDBName)
		}
	}
	return selfUpdateGeo(ctx, paths, geoCfg, *autoYes)
}

func RunGeoCfg(args []string) error {
	paths := newAppPaths()
	fs := flag.NewFlagSet("igeo-cfg", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "usage: igeo-cfg [options]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "geo-cfg 编辑geo config")
		fmt.Fprintln(os.Stdout)
		fs.PrintDefaults()
	}
	example := fs.Bool("e", false, "")
	fs.BoolVar(example, "example", false, "显示配置文件示例")
	if err := fs.Parse(normalizeArgs(args, geoCfgArgSpec())); err != nil {
		return ErrUsage
	}
	if *example {
		fmt.Print(defaultGeoConfig)
		return nil
	}
	path, err := ensureGeoConfig(paths)
	if err != nil {
		return err
	}
	consolePrint(fmt.Sprintf("编辑配置文件 %s", path))
	return openEditor(path)
}

func RunIPFilter(ctx context.Context, args []string) error {
	paths := newAppPaths()
	fs := flag.NewFlagSet("ip-filter", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "usage: ip-filter [options] source [source ...]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "ip-filter: ip 筛选工具")
		fmt.Fprintln(os.Stdout)
		fs.PrintDefaults()
	}
	var whiteList, blockList, preferLocs, preferOrgs, blockOrgs stringList
	var output string
	var onlyV4, onlyV6 bool
	var crSize int
	var resolveThreadNum int
	fs.Var(&whiteList, "w", "偏好ip参数, 可重复传入")
	fs.Var(&whiteList, "white_list", "偏好ip参数, 可重复传入")
	fs.Var(&blockList, "b", "屏蔽ip参数, 可重复传入")
	fs.Var(&blockList, "block_list", "屏蔽ip参数, 可重复传入")
	fs.Var(&preferLocs, "pl", "偏好国家地区, 可重复传入")
	fs.Var(&preferLocs, "prefer_locs", "偏好国家地区, 可重复传入")
	fs.Var(&preferOrgs, "po", "偏好org, 可重复传入")
	fs.Var(&preferOrgs, "prefer_orgs", "偏好org, 可重复传入")
	fs.Var(&blockOrgs, "bo", "屏蔽org, 可重复传入")
	fs.Var(&blockOrgs, "block_orgs", "屏蔽org, 可重复传入")
	fs.BoolVar(&onlyV4, "4", false, "仅筛选ipv4")
	fs.BoolVar(&onlyV4, "only_v4", false, "仅筛选ipv4")
	fs.BoolVar(&onlyV6, "6", false, "仅筛选ipv6")
	fs.BoolVar(&onlyV6, "only_v6", false, "仅筛选ipv6")
	fs.IntVar(&crSize, "cs", 0, "cidr 随机抽样ip数量限制")
	fs.IntVar(&crSize, "cr_size", 0, "cidr 随机抽样ip数量限制")
	fs.IntVar(&resolveThreadNum, "rs", 0, "域名解析线程数")
	fs.IntVar(&resolveThreadNum, "resolve_thread_num", 0, "域名解析线程数")
	fs.StringVar(&output, "o", "", "输出文件")
	fs.StringVar(&output, "output", "", "输出文件")
	if err := fs.Parse(normalizeArgs(args, ipFilterArgSpec())); err != nil {
		return ErrUsage
	}
	if fs.NArg() == 0 {
		return ErrUsage
	}
	cfg := defaultConfig()
	cfg.Runtime.IPSources = uniqueStrings(fs.Args())
	cfg.Runtime.WhiteList = uniqueStrings([]string(whiteList))
	if len(blockList) > 0 && len(cfg.Runtime.WhiteList) > 0 {
		consolePrint("偏好参数与黑名单参数同时存在, 自动忽略黑名单参数!")
	}
	if len(cfg.Runtime.WhiteList) == 0 {
		cfg.Runtime.BlockList = uniqueStrings([]string(blockList))
	}
	cfg.Runtime.PreferLocs = uniqueStrings([]string(preferLocs))
	if len(cfg.Runtime.WhiteList) > 0 {
		consolePrint("白名单参数为:", pyStringList(cfg.Runtime.WhiteList))
	}
	if len(cfg.Runtime.BlockList) > 0 {
		consolePrint("黑名单参数为:", pyStringList(cfg.Runtime.BlockList))
	}
	if len(cfg.Runtime.PreferLocs) > 0 {
		consolePrint("优选地区参数为:", pyStringList(cfg.Runtime.PreferLocs))
	}
	cfg.Runtime.PreferOrgs = uniqueStrings([]string(preferOrgs))
	if len(blockOrgs) > 0 && len(cfg.Runtime.PreferOrgs) > 0 {
		consolePrint("偏好org参数与屏蔽org参数同时存在, 自动忽略屏蔽org参数!")
	}
	if len(cfg.Runtime.PreferOrgs) == 0 {
		cfg.Runtime.BlockOrgs = uniqueStrings([]string(blockOrgs))
	}
	if len(cfg.Runtime.PreferOrgs) > 0 {
		consolePrint("优选org 参数为:", pyStringList(cfg.Runtime.PreferOrgs))
	}
	if len(cfg.Runtime.BlockOrgs) > 0 {
		consolePrint("屏蔽org 参数为:", pyStringList(cfg.Runtime.BlockOrgs))
	}
	cfg.Runtime.OnlyV4 = onlyV4
	cfg.Runtime.OnlyV6 = onlyV6
	if crSize > 0 {
		cfg.CIDRSampleIPNum = crSize
	}
	if resolveThreadNum > 0 {
		cfg.Runtime.ResolveThreadNum = resolveThreadNum
	}
	consolePrint("cidr 抽样ip 个数为:", cfg.CIDRSampleIPNum)
	consolePrint("域名解析线程数为:", cfg.Runtime.ResolveThreadNum)
	geoSvc, err := openGeoService(paths)
	if err != nil {
		return err
	}
	if geoSvc != nil {
		defer geoSvc.Close()
	}
	infos, metrics, err := parseSources(ctx, cfg, geoSvc, true)
	if err != nil {
		return err
	}
	consolePrint(fmt.Sprintf("解析ip 耗时: %s秒", pyFloat(metrics.ResolveSeconds)))
	consolePrint(fmt.Sprintf("获取geo 信息耗时: %s秒", pyFloat(metrics.GeoSeconds)))
	consolePrint(fmt.Sprintf("预处理ip 总计耗时: %s秒", pyFloat(metrics.TotalSeconds)))
	infos = filterByPreferOrgsWithGeo(infos, cfg.Runtime.PreferOrgs, geoSvc)
	infos = filterByBlockOrgsWithGeo(infos, cfg.Runtime.BlockOrgs, geoSvc)
	infos = filterByLocsWithGeo(infos, cfg.Runtime.PreferLocs, geoSvc)
	if len(infos) == 0 {
		consolePrint("未筛选出指定IP, 请检查参数!")
		return nil
	}
	seen := map[string]struct{}{}
	var ips []string
	for _, info := range infos {
		if _, ok := seen[info.IP]; ok {
			continue
		}
		seen[info.IP] = struct{}{}
		ips = append(ips, info.IP)
	}
	consolePrint(fmt.Sprintf("从筛选条件中生成了%d个ip:", len(ips)))
	for _, ip := range ips {
		consolePrint(ip)
	}
	if output != "" {
		if err := os.WriteFile(output, []byte(joinLines(ips)+"\n"), 0o644); err != nil {
			return err
		}
		consolePrint(fmt.Sprintf("筛选通过%d个ip 已导入到%s", len(ips), output))
	}
	return nil
}

func printBetterIPs(infos []IPInfo) {
	consolePrint("优选ip 如下: ")
	for _, info := range infos {
		consolePrint(info.infoString())
	}
}

func writeBetterIPs(infos []IPInfo, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, info := range infos {
		if _, err := fmt.Fprintln(file, info.fileInfoString()); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(file); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(file, generatedTimeDescription()); err != nil {
		return err
	}
	consolePrint(fmt.Sprintf("测试通过%d个优选ip 已导出到 %s", len(infos), path))
	return nil
}

func writePureIPs(infos []IPInfo, path string) error {
	lines := make([]string, 0, len(infos))
	for _, info := range infos {
		lines = append(lines, info.IP)
	}
	if err := os.WriteFile(path, []byte(joinLines(lines)+"\n"), 0o644); err != nil {
		return err
	}
	consolePrint(fmt.Sprintf("测试通过%d个优选ip 已导出到 %s", len(infos), path))
	return nil
}

func downloadFile(ctx context.Context, rawURL, path, proxy string) error {
	consolePrint("正在下载geo database ... ...")
	consolePrint(fmt.Sprintf("下载代理为: %s", proxy))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	client, err := newTimeoutHTTPClient(proxy, 30*time.Second)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: http %d", resp.StatusCode)
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	total := resp.ContentLength
	buf := make([]byte, 32*1024)
	var written int64
	startedAt := time.Now()
	lastUpdate := time.Now()
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := file.Write(buf[:n]); err != nil {
				return err
			}
			written += int64(n)
			if time.Since(lastUpdate) > 150*time.Millisecond {
				printDownloadProgress(path, written, total, startedAt)
				lastUpdate = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	printDownloadProgress(path, written, total, startedAt)
	consoleKeepRefreshLine()
	consolePrint(fmt.Sprintf("下载geo database到%s 成功.", path))
	return nil
}

func selfUpdateGeo(ctx context.Context, paths appPaths, cfg GeoConfig, autoYes bool) error {
	if cfg.DBAPIURL == "" {
		return fmt.Errorf("geo config missing db_api_url")
	}
	consolePrint(fmt.Sprintf("请求代理为: %s", cfg.Proxy))
	client, err := newTimeoutHTTPClient(cfg.Proxy, 15*time.Second)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.DBAPIURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var remote map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&remote); err != nil {
		return err
	}
	local := loadVersionFile(paths.geoVersion)
	localTag, _ := local["tag_name"].(string)
	remoteTag, _ := remote["tag_name"].(string)
	allowUpdate := autoYes
	switch {
	case len(remote) == 0:
		allowUpdate = autoYes || askConfirm("检测GEO数据库更新失败, 是否强制下载GEO数据库: Y(es)/N(o)")
	case localTag != remoteTag:
		if localTag == "" {
			localTag = "unknown"
		}
		if remoteTag == "" {
			remoteTag = "unknown"
		}
		if autoYes {
			consolePrint(fmt.Sprintf("检测到GEO数据库有更新: %s -> %s, 自动更新下载中... ...", localTag, remoteTag))
			allowUpdate = true
		} else {
			allowUpdate = askConfirm(fmt.Sprintf("检测到GEO数据库有更新: %s -> %s, 是否更新: Y(es)/N(o)", localTag, remoteTag))
		}
	default:
		if remoteTag == "" {
			remoteTag = "unknown"
		}
		if autoYes {
			consolePrint(fmt.Sprintf("检测到GEO数据库为最新: %s, 无需更新!", remoteTag))
			return nil
		}
		allowUpdate = askConfirm(fmt.Sprintf("GEO数据库已最新: %s, 是否强制重新下载GEO数据库: Y(es)/N(o)", remoteTag))
	}
	if !allowUpdate {
		return nil
	}
	consolePrint("ASN 数据库下载地址:", cfg.DBASNURL)
	if err := downloadFile(ctx, cfg.DBASNURL, paths.geoASNDB, cfg.Proxy); err != nil {
		return err
	}
	consolePrint("CITY 数据库下载地址:", cfg.DBCityURL)
	if err := downloadFile(ctx, cfg.DBCityURL, paths.geoCityDB, cfg.Proxy); err != nil {
		return err
	}
	if len(remote) == 0 {
		remote = local
	}
	return saveVersionFile(paths.geoVersion, remote)
}

func askConfirm(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprint(os.Stdout, prompt+"\n")
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return false
		}
		answer := strings.TrimSpace(strings.ToUpper(line))
		switch answer {
		case "Y", "YES":
			return true
		case "N", "NO":
			return false
		}
		consolePrint("输入有误, 请重新输入!")
		if errors.Is(err, io.EOF) {
			return false
		}
	}
}

func printDownloadProgress(path string, current, total int64, startedAt time.Time) {
	elapsed := time.Since(startedAt).Seconds()
	speed := int64(0)
	if elapsed > 0 {
		speed = int64(float64(current) / elapsed)
	}
	if total > 0 {
		consoleRefresh("%s 已下载 %s/%s 当前速度 %s/s", path, humanSize(current), humanSize(total), humanSize(speed))
		return
	}
	consoleRefresh("%s 已下载 %s 当前速度 %s/s", path, humanSize(current), humanSize(speed))
}

func humanSize(v int64) string {
	const unit = 1024
	if v < unit {
		return fmt.Sprintf("%dB", v)
	}
	div, exp := int64(unit), 0
	for n := v / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(v)/float64(div), "KMGTPE"[exp])
}

func hasSuffixAny(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for _, line := range lines[1:] {
		out += "\n" + line
	}
	return out
}

func printIPCheckUsage() {
	consolePrint("usage: ip-check [options] source [source ...]")
}

type stringList []string

func (s *stringList) String() string { return fmt.Sprint([]string(*s)) }
func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type intList []int

func (s *intList) String() string { return fmt.Sprint([]int(*s)) }
func (s *intList) Set(value string) error {
	var v int
	_, err := fmt.Sscan(value, &v)
	if err != nil {
		return err
	}
	*s = append(*s, v)
	return nil
}

type argKind int

const (
	argBool argKind = iota
	argSingle
	argMulti
)

type argSpec map[string]argKind

func normalizeArgs(args []string, spec argSpec) []string {
	if len(args) == 0 {
		return nil
	}
	var opts []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if kind, ok := spec[arg]; ok {
			switch kind {
			case argBool:
				opts = append(opts, arg)
			case argSingle:
				opts = append(opts, arg)
				if i+1 < len(args) {
					opts = append(opts, args[i+1])
					i++
				}
			case argMulti:
				for i+1 < len(args) {
					next := args[i+1]
					if next == "--" {
						i++
						positional = append(positional, args[i+1:]...)
						return append(opts, positional...)
					}
					if _, isFlag := spec[next]; isFlag {
						break
					}
					if strings.HasPrefix(next, "-") && !looksLikeValue(next) {
						break
					}
					opts = append(opts, arg, next)
					i++
				}
			}
			continue
		}
		positional = append(positional, arg)
	}
	return append(opts, positional...)
}

func looksLikeValue(arg string) bool {
	if arg == "-" {
		return true
	}
	if _, err := strconv.Atoi(arg); err == nil {
		return true
	}
	return false
}

func ipCheckArgSpec() argSpec {
	return argSpec{
		"-w": argMulti, "--white_list": argMulti,
		"-b": argMulti, "--block_list": argMulti,
		"-pl": argMulti, "--prefer_locs": argMulti,
		"-po": argMulti, "--prefer_orgs": argMulti,
		"-bo": argMulti, "--block_orgs": argMulti,
		"-pp": argMulti, "--prefer_ports": argMulti,
		"-lv": argSingle, "--max_vt_ip_count": argSingle,
		"-lr": argSingle, "--max_rt_ip_count": argSingle,
		"-ls": argSingle, "--max_st_ip_count": argSingle,
		"-lb": argSingle, "--max_bt_ip_count": argSingle,
		"-p": argSingle, "--port": argSingle,
		"-H": argSingle, "--host": argSingle,
		"-dr": argBool, "--disable_rt": argBool,
		"-dv": argBool, "--disable_vt": argBool,
		"-ds": argBool, "--disable_st": argBool,
		"-o": argSingle, "--output": argSingle,
		"-f": argBool, "--fast_check": argBool,
		"-s": argSingle, "--speed": argSingle,
		"-as": argSingle, "--avg_speed": argSingle,
		"-r": argSingle, "--rtt": argSingle,
		"-l": argSingle, "--loss": argSingle,
		"-c": argSingle, "--config": argSingle,
		"-u": argSingle, "--url": argSingle,
		"-v": argBool, "--verbose": argBool,
		"-ns": argBool, "--no_save": argBool,
		"--dry_run": argBool,
		"-4": argBool, "--only_v4": argBool,
		"-6": argBool, "--only_v6": argBool,
		"-cs": argSingle, "--cr_size": argSingle,
		"-rs": argSingle, "--resolve_thread_num": argSingle,
		"-df": argBool, "--disable_file_check": argBool,
		"--pure_mode": argBool,
		"--version": argBool,
	}
}

func ipFilterArgSpec() argSpec {
	return argSpec{
		"-w": argMulti, "--white_list": argMulti,
		"-b": argMulti, "--block_list": argMulti,
		"-pl": argMulti, "--prefer_locs": argMulti,
		"-po": argMulti, "--prefer_orgs": argMulti,
		"-bo": argMulti, "--block_orgs": argMulti,
		"-4": argBool, "--only_v4": argBool,
		"-6": argBool, "--only_v6": argBool,
		"-cs": argSingle, "--cr_size": argSingle,
		"-rs": argSingle, "--resolve_thread_num": argSingle,
		"-o": argSingle, "--output": argSingle,
	}
}

func geoInfoArgSpec() argSpec {
	return argSpec{}
}

func pyBool(v bool) string {
	if v {
		return "True"
	}
	return "False"
}

func pyStringList(items []string) string {
	quoted := make([]string, 0, len(items))
	for _, item := range items {
		quoted = append(quoted, fmt.Sprintf("'%s'", item))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func pyIntList(items []int) string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, strconv.Itoa(item))
	}
	return "[" + strings.Join(out, ", ") + "]"
}

func shuffleIPInfos(infos []IPInfo) {
	seededRand.Shuffle(len(infos), func(i, j int) {
		infos[i], infos[j] = infos[j], infos[i]
	})
}

func geoDownloadArgSpec() argSpec {
	return argSpec{
		"-u": argSingle, "--url": argSingle,
		"-p": argSingle, "--proxy": argSingle,
		"-y": argBool, "--yes": argBool,
	}
}

func geoCfgArgSpec() argSpec {
	return argSpec{
		"-e": argBool, "--example": argBool,
	}
}

func cfgArgSpec() argSpec {
	return argSpec{
		"-o": argSingle, "--output": argSingle,
		"-e": argBool, "--example": argBool,
	}
}

func checkGeoUpdate(ctx context.Context, paths appPaths, geoCfg GeoConfig, updateChan chan<- string) {
	if geoCfg.DBAPIURL == "" {
		return
	}
	client, err := newTimeoutHTTPClient(geoCfg.Proxy, 10*time.Second)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geoCfg.DBAPIURL, nil)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var remote map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&remote); err != nil {
		return
	}
	local := loadVersionFile(paths.geoVersion)
	localTag, _ := local["tag_name"].(string)
	remoteTag, _ := remote["tag_name"].(string)
	if remoteTag != "" && localTag != remoteTag {
		if localTag == "" {
			localTag = "unknown"
		}
		updateChan <- fmt.Sprintf("\n[notice] A new release of geo database is available: %s -> %s\n[notice] To update, run: igeo-dl", localTag, remoteTag)
	}
}
