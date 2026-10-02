# 山特 UPS → 群晖 NUT 网络桥接器

这是一个小型、跨平台的 UPS 状态桥接服务。它通过串口读取山特 UPS 的 Megatec/Q1 状态，并在 TCP `3493` 上提供兼容群晖 DSM 的 NUT 1.3 服务，让 DSM 能够识别市电中断和低电量状态并执行安全关机。

程序只发送只读的 `Q1\r` 查询，不向 UPS 发送关机、自检、蜂鸣器或负载控制命令。局域网客户端也不能通过本程序触发 UPS 或 NAS 关机。

## 设备兼容性

### 已实测 UPS

| 项目 | 实测值 |
|---|---|
| 品牌/协议 | 山特，Megatec/Q1 串口协议 |
| USB VID:PID | `15a2:0039` |
| Windows 串口 | `COM9` |
| Linux 常见设备 | `/dev/ttyACM0` |
| 串口参数 | `2400 baud, 8 data bits, no parity, 1 stop bit` |
| 查询命令 | `Q1\r` |
| 响应样例 | `(231.3 000.0 220.5 016 49.9 2.27 36.0 00000000` |

### 可能兼容的 UPS

只有同时满足以下条件的型号才可能兼容：

1. 系统能把 UPS 暴露为可读写串口（如 `COMx`、`/dev/ttyACM0` 或 `/dev/ttyUSB0`）。
2. 串口使用 `2400-8-N-1`。
3. 发送 `Q1\r` 后返回以 `(` 开头、包含 7 个数值字段和 8 位状态标志的 Megatec Q1 数据。

“山特”品牌本身不代表一定兼容。纯 USB HID 型号、只支持其他 Megatec 命令/波特率的型号、不会返回上述 Q1 格式的型号均不兼容。使用未实测型号前，先执行 `-once`；只有成功输出 JSON 后再连接 DSM。

### 运行平台与预编译文件

| 平台 | 预编译文件 | 典型设备 |
|---|---|---|
| Windows x64 | `dist/santak-nut-bridge-windows-amd64.exe` | Windows 10/11 电脑 |
| Linux x86_64 | `dist/santak-nut-bridge-linux-amd64` | x86 群晖、软路由、服务器 |
| Linux ARM64 | `dist/santak-nut-bridge-linux-arm64` | ARM64 NAS、树莓派、OpenWrt |
| Linux ARMv7 | `dist/santak-nut-bridge-linux-armv7` | 32 位 ARM 路由器/NAS |
| Linux MIPS little-endian soft-float | `dist/santak-nut-bridge-linux-mipsle-softfloat` | 部分老款 OpenWrt 路由器 |

Docker 方式适用于能够把 USB 串口设备映射进 Linux 容器的 x86_64/ARM64 NAS 或服务器。`deploy/synology/` 中的原生接管脚本按 DSM 7 的 systemd 服务和 `/volume1/docker/santak-nut-bridge` 路径编写；其他卷号、旧版 DSM 或不同服务结构需要先修改脚本，不能直接套用。

## 部署位置怎么选

程序必须运行在 **UPS USB 线实际连接的设备** 上：

- USB 接 Windows：运行 Windows 版，DSM 填写这台电脑的固定局域网 IP。电脑必须保持开机。
- USB 接群晖：使用 DSM 原生接管脚本，或通过 Container Manager/Docker 运行。
- USB 接 OpenWrt：安装 USB 串口驱动，使用与路由器 CPU 架构相符的二进制和 procd 服务。
- USB 接其他 Linux 主机：直接运行对应二进制或使用 Docker，DSM 填写该主机的固定 IP。

DSM 会主动连接指定 IP 的 TCP `3493`，不需要广播发现。

## 第一步：确认 UPS 能读取

在启动 NUT 服务前，先做一次只读查询。

Windows：

```powershell
.\dist\santak-nut-bridge-windows-amd64.exe -serial COM9 -once
```

不知道串口号时可自动探测：

```powershell
.\dist\santak-nut-bridge-windows-amd64.exe -serial auto -once
```

Linux：

```sh
chmod +x ./dist/santak-nut-bridge-linux-amd64
./dist/santak-nut-bridge-linux-amd64 -serial /dev/ttyACM0 -once
```

成功时会输出包含 `ups.status`、输入/输出电压、负载、电池电压和温度的 JSON。失败时先检查串口路径、USB 驱动、设备权限，以及是否被 DSM 自带 `usbhid-ups` 或其他监控程序占用。

## 方案一：Windows 运行

启动常驻服务：

```powershell
.\dist\santak-nut-bridge-windows-amd64.exe -serial COM9
```

日志出现 `UPS connected` 后，保持窗口和电脑运行。只允许 NAS 的固定 IP 访问端口（把示例地址替换为实际 NAS IP，并以管理员身份执行）：

```powershell
New-NetFirewallRule -DisplayName "SANTAK NUT from Synology" -Direction Inbound -Action Allow -Protocol TCP -LocalPort 3493 -RemoteAddress 192.168.1.20
```

如果希望开机自动运行，可使用 Windows 任务计划程序，以“无论用户是否登录都运行”方式启动该命令。

## 方案二：群晖 Container Manager / Docker

1. 把项目复制到群晖，例如 `/volume1/docker/santak-nut-bridge`。
2. 将 UPS USB 接到群晖。
3. SSH 登录群晖并查找串口：

```sh
ls -l /dev/ttyACM* /dev/ttyUSB*
```

4. 如果不是 `/dev/ttyACM0`，复制并修改配置：

```sh
cp .env.example .env
```

```dotenv
UPS_DEVICE=/dev/ttyUSB0
```

5. 启动并检查日志：

```sh
docker compose up -d --build
docker compose logs -f
```

日志应出现 `UPS connected`。如果主机没有生成 `/dev/ttyACM*` 或 `/dev/ttyUSB*`，通常说明该 DSM 内核没有加载对应的 CDC-ACM/USB 串口驱动；这时把 USB 接到 Windows、OpenWrt 或其他 Linux 主机通常更省事。

> Docker 容器以 root 运行，仅用于访问 NAS 上权限不统一的串口设备。不要把 TCP `3493` 暴露到公网。

## 方案三：接管群晖 DSM 原生 UPS 服务

此方式用桥接器替代 DSM 对该 USB 设备不兼容的 `usbhid-ups` 驱动，同时继续使用 DSM 自带的 `upsmon`、通知和安全关机流程。

使用前请确认：DSM 使用 systemd；项目位于 `/volume1/docker/santak-nut-bridge`；串口为 `/dev/ttyACM0`。安装脚本要求二进制和 5 个部署文件位于该目录顶层；路径不同时应先修改脚本和 `override.conf`。

```sh
cd /volume1/docker/santak-nut-bridge
cp dist/santak-nut-bridge-linux-amd64 santak-nut-bridge   # 按群晖 CPU 架构选择
cp deploy/synology/install.sh deploy/synology/uninstall.sh deploy/synology/prepare.sh deploy/synology/run.sh deploy/synology/override.conf .
chmod 755 santak-nut-bridge install.sh uninstall.sh prepare.sh run.sh
sudo ./install.sh
sudo systemctl status ups-usb.service
/usr/bin/upsc ups@localhost
```

安装器只增加 `ups-usb.service` 的 systemd drop-in，不修改原厂 unit 文件。撤销接管并恢复 DSM 原始服务：

```sh
sudo /volume1/docker/santak-nut-bridge/uninstall.sh
```

脚本会停止并替换正在运行的 DSM UPS 服务。请在 NAS 无重要写入任务、有人在场且有备用访问方式时操作。DSM 升级后应重新检查 drop-in 和服务状态。

## 方案四：OpenWrt

安装 CDC-ACM 驱动并确认设备：

```sh
opkg update
opkg install kmod-usb-acm
ls -l /dev/ttyACM*
ubus call system board
opkg print-architecture
```

根据 CPU 选择 `dist/` 中的二进制：常见 x86_64 用 `linux-amd64`，64 位 ARM 用 `linux-arm64`，32 位 ARMv7 用 `linux-armv7`。

```sh
scp dist/santak-nut-bridge-linux-arm64 root@路由器IP:/usr/bin/santak-nut-bridge
scp deploy/openwrt/santak-nut-bridge.init root@路由器IP:/etc/init.d/santak-nut-bridge
scp deploy/openwrt/santak-nut-bridge.config root@路由器IP:/etc/config/santak-nut-bridge
ssh root@路由器IP 'chmod 755 /usr/bin/santak-nut-bridge /etc/init.d/santak-nut-bridge && /etc/init.d/santak-nut-bridge enable && /etc/init.d/santak-nut-bridge start'
```

查看日志：

```sh
logread -e santak-nut-bridge
```

只允许群晖所在 LAN 访问 TCP `3493`，不要向 WAN 开放。

## 群晖 DSM 设置

当桥接器运行在 Windows、OpenWrt 或另一台主机时：

1. 打开“控制面板 → 硬件和电源 → UPS”。
2. 勾选“启用 UPS 支持”。
3. 网络 UPS 类型选择“Synology UPS 服务器”。
4. 服务器地址填写运行桥接器设备的 **固定局域网 IP**。
5. 设置断电后进入待机/安全模式的等待时间并应用。

程序默认使用 DSM 兼容值：UPS 名称 `ups`、用户 `monuser`、密码 `secret`。部分 DSM 客户端固定使用这些值，修改后可能无法连接。NUT 1.3 登录和数据均为明文，因此安全措施应依赖可信 LAN/VLAN 和来源 IP 防火墙规则。

## 验证 NUT 服务

使用 NUT 客户端：

```sh
upsc ups@服务器IP
```

或使用 `nc`：

```text
$ nc 服务器IP 3493
LIST UPS
GET VAR ups ups.status
GET VAR ups battery.charge
```

常见状态：

| 状态 | 含义 |
|---|---|
| `OL` | 市电正常（On Line） |
| `OB` | 电池供电（On Battery） |
| `LB` | UPS 硬件报告低电量（Low Battery） |
| `FSD` | 强制关机状态；仅允许本机 DSM `upsmon` 设置 |

进行断电测试时，只拔 UPS 的市电输入，**不要拔 USB 线，也不要断开 NAS 所接的 UPS 输出**。`ups.status` 应从 `OL` 变为 `OB`，恢复市电后回到 `OL`。首次测试务必有人在场、保存数据并确认 NAS 能够正常恢复。

## 配置项

所有命令行参数都有同名环境变量：

| 参数 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `-serial` | `UPS_SERIAL` | Windows `COM9`；Linux `/dev/ttyACM0` | 串口路径；可用 `auto` 自动探测 |
| `-listen` | `UPS_LISTEN` | `0.0.0.0:3493` | NUT 监听地址 |
| `-name` | `UPS_NAME` | `ups` | DSM 期望的 UPS 名称 |
| `-user` | `UPS_USER` | `monuser` | DSM 兼容用户名 |
| `-password` | `UPS_PASSWORD` | `secret` | DSM 兼容密码 |
| `-manufacturer` | `UPS_MANUFACTURER` | `SANTAK` | 发布给 NUT 客户端的厂商名 |
| `-model` | `UPS_MODEL` | `Megatec/Q1 UPS` | 发布给 NUT 客户端的型号名 |
| `-poll` | `UPS_POLL` | `5s` | 串口轮询间隔，最小 `1s` |
| `-stale-after` | `UPS_STALE_AFTER` | `30s` | 超过此时间未更新则报告数据过期，必须大于轮询间隔 |
| `-battery-low` | `UPS_BATTERY_LOW` | `1.75` | 估算为 0% 的原始单节电压 |
| `-battery-high` | `UPS_BATTERY_HIGH` | `2.30` | 估算为 100% 的原始单节电压 |
| `-once` | 无 | 关闭 | 查询一次、输出 JSON 后退出 |
| `-version` | 无 | 关闭 | 输出版本号后退出 |

## 数据映射与限制

- Q1 市电中断位映射为 `OB`，低电量位映射为 `LB`。
- 输入/输出电压、频率、负载、温度和电池电压发布为标准 NUT 变量。
- 实测 UPS 不支持 `F` 额定值查询，Q1 返回的 `2.27V` 是原始单节电压。
- `battery.charge` 只是按 `battery-low` 到 `battery-high` 线性估算；真正决定安全关机的是 UPS 硬件提供的 `LB` 状态。
- UPS 不提供真实的剩余运行时间，因此没有 `battery.runtime`。DSM 日志可能提示一次该变量不受支持，不影响 `OL`、`OB`、`LB` 监控和安全关机。
- 远程 `FSD`、`SET` 和 `INSTCMD` 被拒绝；只有回环地址上的 DSM `upsmon` 可以设置逻辑 `FSD`。

## 常见故障

**`no serial ports found` 或无法打开设备**  
检查 USB 线、驱动、串口名称和用户权限；Linux 可运行 `dmesg | tail`，Windows 可在设备管理器中查看 COM 号。

**`Q1 response timed out`**  
设备可能不是 Q1 协议、波特率不符、串口被其他程序占用，或 UPS USB 适配器需要不同控制线状态。

**日志反复重连**  
检查 USB 供电/线材、虚拟机或容器的 USB 透传是否稳定，以及是否有第二个 UPS 驱动同时读取同一串口。

**DSM 显示无法连接网络 UPS**  
先用 `upsc`/`nc` 测试 TCP `3493`，再检查 IP 是否固定、防火墙是否允许 NAS 来源地址、UPS 名是否仍为 `ups`。

## 从源码构建

需要 Go 1.22 或更高版本：

```powershell
go test ./...
.\scripts\build.ps1
```

Linux/macOS：

```sh
go test ./...
./scripts/build.sh
```

构建脚本会生成 `dist/` 中列出的多个平台文件，并写入 `SHA256SUMS`。

## 许可证

本项目使用 MIT License，详见 `LICENSE`。
