## 【置顶】 厚脸皮求个人人人药，预注册id:bruceplus 邮箱：bruceplus@qq.com

# QBittorrent 智能限速控制器使用文档

## 简介

本程序是一个用于动态调整 QBittorrent 上传速率的工具，旨在根据外网流量的实时情况优化家庭网络性能。通过配置文件，您可以自定义运行参数。

诞生背景：本人用的是飞牛系统，需要PT保种，但又希望外网访问时优先保障观影体验。
外网访问我是通过Lucky做了配置，因此根据Lucky流量来动态调整QBittorrent上行速率

默认查询的是Lucky进程，也可以修改为其他进程名，但没有经过测试，自行使用！！！

初始一个二进制文件，运行后会创建一个日志文件及配置文件。配置文件内容为空会使用默认配置

已测试场景：linux-amd64

## 使用方式一：直接使用

将编译的二进制文件通过ssh上传后，cd到当前目录。赋权后执行

```shell
chmod +x speed-limit
./speed-limit
```

## 使用方式二：通过`systemd`管理，推荐

```bash
chmod +x speed-limit
```

```bash
bruceplus@HomeNas:~$ sudo nano /etc/systemd/system/speed-limit.service
将以下内容粘贴到编辑器中，并ctrl+o保存，ctrl+X关闭退出
[Unit]
Description=Speed Limit Script
After=network.target

[Service]
Type=simple
#二进制文件路径
ExecStart=/vol1/1000/script/speedlimit/speed-limit
#文件目录
WorkingDirectory=/vol1/1000/script/speedlimit/
#日志路径
StandardOutput=append:/vol1/1000/script/speedlimit/qbittorrent_limit.log
StandardError=append:/vol1/1000/script/speedlimit/qbittorrent_limit.log
#这里我关掉了自动重启Restart=no，有需要的可以打开。风险：qb登陆失败会重启，尝试多次登录导致被封禁，解决办法重启qb
Restart=on-failure
#这里不清楚自己用户名和用户组的执行这个命令查看：echo "用户名: $(whoami), 主用户组: $(id -gn)"
User=bruceplus
Group=Users

[Install]
WantedBy=multi-user.target

```

保存后重新加载并运行

```bash
#重新加载 systemd 的配置
bruceplus@HomeNas:~$ sudo systemctl daemon-reload
#系统启动时自动启动,但是飞牛系统更新后没有生效
bruceplus@HomeNas:~$ sudo systemctl enable speed-limit
#立即启动 speed-limit 服务
bruceplus@HomeNas:~$ sudo systemctl start speed-limit
Created symlink /etc/systemd/system/multi-user.target.wants/speed-limit.service → /etc/systemd/system/speed-limit.service.
```

---

## 配置文件说明

配置文件名：`config.json`

配置文件采用 JSON 格式，第一次启动会创建config.json配置文件。以下是各项配置的详细说明：

### 配置项一览表

| 配置项名                    | 类型     | 默认值                     | 说明                   |
|-------------------------|--------|-------------------------|----------------------|
| `qbittorrent_url`       | 字符串    | `http://localhost:8085` | QBittorrent WebUI 地址 |
| `username`              | 字符串    | `admin`                 | QBittorrent 用户名      |
| `password`              | 字符串    | `adminadmin`            | QBittorrent 密码       |
| `total_bandwidth`       | 整数（字节） | `31457280` （30MB/s）     | 家庭网络上行总带宽（单位：字节）     |
| `rate_limit_offset`     | 整数（字节） | `1048576` （1MB/s）       | 限速偏移量（单位：字节）         |
| `threshold`             | 整数（字节） | `102400` （100KB）        | 流量阈值，超过该值时限速         |
| `samples_per_period`    | 整数     | `15`                    | 每个采样周期的采样次数          |
| `check_interval`        | 字符串    | `@every 2s`             | 流量检查间隔（Cron 表达式）     |
| `limit_adjust_interval` | 字符串    | `@every 30s`            | 限速调整间隔（Cron 表达式）     |
| `log_path`              | 字符串    | `qbittorrent_limit.log` | 日志文件路径               |
| `monitor_process`       | 字符串    | `Lucky`                 | 被监控的进程名称             |

---

### 配置项详解

#### 1. `qbittorrent_url`

- **类型**: 字符串
- **说明**: QBittorrent 的 WebUI 地址，通常是 `http://<主机>:<端口>`。
- **示例**:
  ```json
  "qbittorrent_url": "http://192.168.1.100:8085"
  ```

#### 2. `username`

- **类型**: 字符串
- **说明**: QBittorrent WebUI 的登录用户名。
- **示例**:
  ```json
  "username": "admin"
  ```

#### 3. `password`

- **类型**: 字符串
- **说明**: QBittorrent WebUI 的登录密码。
- **示例**:
  ```json
  "password": "mypassword"
  ```

#### 4. `total_bandwidth`

- **类型**: 整数（字节）
- **说明**: 家庭网络的总带宽上限（单位为字节）。
- **建议**: 根据您网络的实际速度设置，带宽值 = 速度（MB/s） × 1024 × 1024。
- **示例**:
  ```json
  "total_bandwidth": 10485760 // 10MB/s
  ```

#### 5. `threshold`

- **类型**: 整数（字节）
- **说明**: 当外网流量超过此阈值时，开始限速。
- **示例**:
  ```json
  "threshold": 102400 // 100KB
  ```

#### 6. `samples_per_period`

- **类型**: 整数
- **说明**: 每个采样周期内的采样次数。最小值为 1。
- **示例**:
  ```json
  "samples_per_period": 20
  ```

#### 7. `check_interval`

- **类型**: 字符串
- **说明**: 采样任务的执行间隔，使用 Cron 表达式格式。
- **示例**:
  ```json
  "check_interval": "@every 5s" // 每 5 秒采样一次
  ```

#### 8. `limit_adjust_interval`

- **类型**: 字符串
- **说明**: 限速任务的调整间隔，使用 Cron 表达式格式。
- **示例**:
  ```json
  "limit_adjust_interval": "@every 1m" // 每分钟调整一次限速
  ```

#### 9. `log_path`

- **类型**: 字符串
- **说明**: 程序运行日志的存储路径。
- **示例**:
  ```json
  "log_path": "path/to/logfile.log"
  ```

#### 10. `monitor_process`

- **类型**: 字符串
- **说明**: 被监控的进程名称，用于动态获取外网流量。
- **示例**:
  ```json
  "monitor_process": "Lucky"
  ```

---

#### 11. `rate_limit_offset`

- **类型**: 整数（字节）
- **说明**:
  限速偏移量（单位为字节）。默认值为0。若当前QB的上行速率限制（如3MB/s）仍不符合需求，您可通过调整该值进一步降低限速。例如，将rate_limit_offset设置为1 ×
  1024 × 1024(1M/S)，则速率限制将降至2MB/s。
- **建议**: 根据您网络的实际速度设置，带宽值 = 速度（MB/s） × 1024 × 1024。
- **示例**:
  ```json
  "rate_limit_offset": 1048576 // 1MB/s
  ```

## 配置文件生成

如果配置文件不存在，程序会自动生成默认配置文件。默认路径为 `config.json`，文件内容如下：

```json
{
  "qbittorrent_url": "http://localhost:8085",
  "username": "admin",
  "password": "adminadmin",
  "total_bandwidth": 31457280,
  "threshold": 102400,
  "samples_per_period": 15,
  "check_interval": "@every 2s",
  "limit_adjust_interval": "@every 30s",
  "log_path": "qbittorrent_limit.log",
  "monitor_process": "Lucky"
}
```

---

## 注意事项

1. **权限问题**:
    - 确保程序运行时有权限读取 `/proc/<PID>/net/dev` 文件。
2. **日志查看**:
    - 程序运行日志会记录在 `log_path` 所指定的文件中，可用于排查问题。
3. **Cron 表达式**:
    - Cron
      表达式可以灵活控制任务执行的时间间隔，具体语法参考 [Cron 表达式官方文档](https://pkg.go.dev/github.com/robfig/cron/v3#hdr-CRON_Expression_Parser)。
4. **配置更新**:
    - 修改配置文件后需重启程序以生效。

---

## 常见问题

### 1. 程序无法检测到外网流量？

- 检查配置的 `monitor_process` 是否正确。
- 确保相关进程正在运行。

### 2. QBittorrent 限速未生效？

- 确保 WebUI 地址和登录信息配置正确。
- 检查日志文件是否记录了限速调整操作。

### 3. 如何调试程序？

- 查看日志文件，查找 `[QB-Limit]` 标记的日志信息。

---

通过正确配置和使用该程序，可以更好地优化您的家庭网络，确保在下载和日常使用之间取得平衡！

"# qb-limit"  
