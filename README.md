# ip-check-go

`ip-check` 的 Go 版本，现已改成多命令入口结构，覆盖原仓库的核心功能：

- 配置解析
- IP / CIDR / `ip:port` / 域名输入
- GEO 信息查询
- 可用性检测
- RTT 测试
- 单线程下载测速
- `ip-filter`
- `igeo-info`
- `igeo-dl`
- `ip-check-cfg`
- `igeo-cfg`

## Notice

该仓库属于 vibe coding 产物，未经严格测试，使用存在风险，请自行评估。

## 依赖

- Go `1.24+`

## 编译

先拉依赖：

```bash
cd ip-check-go
go mod tidy
```

最方便的是直接一次性编译全部命令：

```bash
cd ip-check-go
make build
```

这会生成：

- `bin/ip-check`
- `bin/ip-check-cfg`
- `bin/igeo-info`
- `bin/igeo-dl`
- `bin/igeo-cfg`
- `bin/ip-filter`
- `bin/config-ex.ini`
- `bin/geo-ex.ini`

如果你想自己指定目标平台和输出目录，直接传 `GOOS`、`GOARCH`、`OUT_DIR`：

```bash
make build GOOS=linux GOARCH=arm64 OUT_DIR=dist/linux-arm64
make build GOOS=windows GOARCH=amd64 OUT_DIR=dist/windows-amd64
```

如果不指定 `GOOS` / `GOARCH`，默认就是当前系统环境。

当前 Makefile 帮助里列出的支持值为：

- `GOOS`: `linux`, `windows`
- `GOARCH`: `amd64`, `arm64`

如果你要一次性编译多平台：

```bash
make build-all
```

会输出到这些目录：

- `bin/linux-amd64/`
- `bin/linux-arm64/`
- `bin/windows-amd64/`

也可以单独编译指定平台：

```bash
make build GOOS=linux GOARCH=amd64 OUT_DIR=bin/linux-amd64
make build GOOS=linux GOARCH=arm64 OUT_DIR=bin/linux-arm64
make build GOOS=windows GOARCH=amd64 OUT_DIR=bin/windows-amd64
```

如果你不想用 `make`，也可以分别编译六个命令：

```bash
go build -o bin/ip-check ./cmd/ip-check
go build -o bin/ip-check-cfg ./cmd/ip-check-cfg
go build -o bin/igeo-info ./cmd/igeo-info
go build -o bin/igeo-dl ./cmd/igeo-dl
go build -o bin/igeo-cfg ./cmd/igeo-cfg
go build -o bin/ip-filter ./cmd/ip-filter
```

如果你只是临时测试，也可以分别运行：

```bash
go run ./cmd/ip-check -- test.txt
go run ./cmd/igeo-info -- 1.1.1.1
go run ./cmd/ip-filter -- test.txt -o out.txt
```

## 配置文件

程序默认使用“命令所在目录”：

- 直接读取可执行文件所在目录里的 `config.ini` / `geo.ini`
- 默认配置由同目录下的 `config-ex.ini` / `geo-ex.ini` 原样复制生成
- 如果后续你希望固定目录，再设置 `IPCHECK_HOME`

首次运行会自动生成：

- `config.ini`
- `geo.ini`
- `config-ex.ini`
- `geo-ex.ini`

GEO 数据库默认也放在同目录：

- `GeoLite2-City.mmdb`
- `GeoLite2-ASN.mmdb`

## 常用命令

```bash
./bin/ip-check test.txt
./bin/ip-check 1.1.1.1/24 -p 443 -s 5000 -r 300
./bin/ip-check test.txt -dv
./bin/ip-check test.txt -dr
./bin/ip-check test.txt -u https://speed.cloudflare.com/__down?bytes=500000000
./bin/ip-filter test.txt -o filtered.txt
./bin/igeo-info 1.1.1.1 8.8.8.8
./bin/igeo-dl -y
./bin/ip-check-cfg
./bin/igeo-cfg
```

## 说明

- HTTPS 检测与测速采用“连接目标 IP，但保留 URL Host 和 SNI”的方式实现。
- 当前 `string` 列表类参数建议重复传入，例如 `-w 1.1 -w 1.2`、`-pl hongkong -pl japan`。
- 单线程测速保持与原 Python 版本接近，逐个 IP 顺序测速。

## License

MIT
