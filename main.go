/*
QBittorrent智能限速控制器
功能：根据外网流量动态调整QBittorrent上传速率，保障家庭网络通畅
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/robfig/cron/v3"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

// Config 配置文件结构体
type Config struct {
	QBittorrentURL string `json:"qbittorrent_url"` // WebUI地址
	Username       string `json:"username"`        // 用户名
	Password       string `json:"password"`        // 密码
	TotalBandwidth int64  `json:"total_bandwidth"` // 总带宽(字节)

	Threshold           int64  `json:"threshold"`             // 流量阈值(字节)
	RateLimitOffset     int64  `json:"rate_limit_offset  "`   // 限速偏移量(字节)
	SamplesPerPeriod    int    `json:"samples_per_period"`    // 采样次数
	CheckInterval       string `json:"check_interval"`        // 检查间隔
	LimitAdjustInterval string `json:"limit_adjust_interval"` // 限速调整间隔
	LogPath             string `json:"log_path"`              // 日志文件路径
	MonitorProcess      string `json:"monitor_process"`       // 监控进程名称
}

// 全局变量
var (
	sid                 string                              // QBittorrent会话ID
	logger              *log.Logger                         // 日志记录器
	appConfig           Config                              // 应用配置
	configPath          = "config.json"                     // 配置文件路径
	QBLoginURL          = "/api/v2/auth/login"              //QBittorrent设置登录url
	QBSetUploadLimitURL = "/api/v2/transfer/setUploadLimit" //QBittorrent设置上传限速url
	defaultCfg          = Config{                           // 默认配置
		QBittorrentURL:      "http://localhost:8085",
		Username:            "admin",
		Password:            "adminadmin",
		TotalBandwidth:      1024 * 1024 * 30, // 30MB/s
		Threshold:           1024 * 100,       // 100KB
		SamplesPerPeriod:    15,
		CheckInterval:       "@every 2s",
		LimitAdjustInterval: "@every 30s",
		LogPath:             "qbittorrent_limit.log",
		MonitorProcess:      "Lucky", // 默认监控Lucky进程
		RateLimitOffset:     0,       // 限速偏移量(字节)
	}
)

// 初始化函数
func init() {
	loadConfig() // 加载配置
	initLogger() // 初始化日志
}

func main() {
	defer logger.Writer().(*os.File).Close()

	// QBittorrent登录认证
	var err error
	if sid, err = qbLogin(); err != nil {
		logger.Fatalf("QBittorrent登录失败: %v", err)
	}

	// 初始化定时任务
	c := cron.New()
	if err := setupCronJobs(c); err != nil {
		logger.Fatalf("定时任务初始化失败: %v", err)
	}
	c.Start()
	defer c.Stop()

	// 优雅关机处理
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	logger.Println("正在关闭服务...")
}

// 在loadConfig函数中增加完整默认值处理
func loadConfig() {
	// 尝试创建默认配置（如果配置文件不存在）
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		createDefaultConfig()
	}

	// 读取配置文件
	file, err := os.Open(configPath)
	if err != nil {
		logger.Fatalf("配置文件打开失败: %v", err)
	}
	defer file.Close()

	// 解析配置
	if err := json.NewDecoder(file).Decode(&appConfig); err != nil {
		logger.Fatalf("配置文件解析失败: %v", err)
	}

	// 为每个配置项设置默认值
	setDefaultStr(&appConfig.QBittorrentURL, defaultCfg.QBittorrentURL, "QBittorrent地址")
	setDefaultStr(&appConfig.Username, defaultCfg.Username, "用户名")
	setDefaultStr(&appConfig.Password, defaultCfg.Password, "密码")
	setDefaultStr(&appConfig.MonitorProcess, defaultCfg.MonitorProcess, "监控进程名")
	setDefaultInt(&appConfig.TotalBandwidth, defaultCfg.TotalBandwidth, "总带宽")
	setDefaultInt(&appConfig.RateLimitOffset, defaultCfg.RateLimitOffset, "限速偏移量")
	setDefaultInt(&appConfig.Threshold, defaultCfg.Threshold, "流量阈值")
	setDefaultStr(&appConfig.CheckInterval, defaultCfg.CheckInterval, "检查间隔")
	setDefaultStr(&appConfig.LimitAdjustInterval, defaultCfg.LimitAdjustInterval, "限速间隔")
	setDefaultStr(&appConfig.LogPath, defaultCfg.LogPath, "日志路径")

	// 特殊校验：采样次数不能小于1
	if appConfig.SamplesPerPeriod < 1 {
		logger.Printf("采样次数配置错误(%d)，重置为默认值%d",
			appConfig.SamplesPerPeriod, defaultCfg.SamplesPerPeriod)
		appConfig.SamplesPerPeriod = defaultCfg.SamplesPerPeriod
	}
}

// 通用默认值设置工具函数
func setDefaultStr(current *string, defaultValue string, fieldName string) {
	if *current == "" {
		logger.Printf("%s使用默认值: %s", fieldName, defaultValue)
		*current = defaultValue
	}
}

func setDefaultInt(current *int64, defaultValue int64, fieldName string) {
	if *current <= 0 {
		logger.Printf("%s使用默认值: %d", fieldName, defaultValue)
		*current = defaultValue
	}
}

// createDefaultConfig 创建默认配置文件
func createDefaultConfig() {
	file, err := os.Create(configPath)
	if err != nil {
		log.Fatalf("配置文件创建失败: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(defaultCfg); err != nil {
		log.Fatalf("默认配置写入失败: %v", err)
	}
}

// initLogger 初始化日志系统
func initLogger() {
	file, err := os.OpenFile(appConfig.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("日志文件初始化失败: %v", err)
	}
	logger = log.New(file, "[QB-Limit] ", log.LstdFlags|log.Lshortfile)
}

// setupCronJobs 配置定时任务
func setupCronJobs(c *cron.Cron) error {
	// 获取QB进程网络统计
	pid, err := getPidByName()
	if err != nil {
		return fmt.Errorf("进程获取失败: %w", err)
	}
	procNetDevPath := fmt.Sprintf("/proc/%s/net/dev", pid)

	// 网络统计文件检查
	if _, err := os.Stat(procNetDevPath); err != nil {
		return fmt.Errorf("网络统计文件不可用: %w", err)
	}

	var (
		prevRx, prevTx int64 // 历史网络数据
		maxTxRate      int64 // 周期内最大速率
	)

	// 网络速率采样任务
	if _, err := c.AddFunc(appConfig.CheckInterval, func() {
		if err := monitorNetwork(procNetDevPath, &prevRx, &prevTx, &maxTxRate); err != nil {
			logger.Fatalf("网络监控异常: %v", err)
		}
	}); err != nil {
		return fmt.Errorf("采样任务创建失败: %w", err)
	}

	// 限速调整任务
	if _, err := c.AddFunc(appConfig.LimitAdjustInterval, func() {
		adjustSpeedLimit(maxTxRate)
		maxTxRate = 0 // 重置最大值
	}); err != nil {
		return fmt.Errorf("限速任务创建失败: %w", err)
	}

	return nil
}

// monitorNetwork 监控网络流量变化
func monitorNetwork(procPath string, prevRx, prevTx, maxTx *int64) error {
	content, err := os.ReadFile(procPath)
	if err != nil {
		return fmt.Errorf("网络统计读取失败: %w", err)
	}

	// 解析网络数据
	lines := strings.Split(string(content), "\n")
	if len(lines) < 3 {
		return fmt.Errorf("网络数据格式错误")
	}
	fields := strings.Fields(lines[2])
	if len(fields) < 10 {
		return fmt.Errorf("网络数据字段不足")
	}

	// 计算实时速率
	currentTx, _ := strconv.ParseInt(fields[9], 10, 64)
	txRate := currentTx - *prevTx
	*prevTx = currentTx

	// 更新周期最大值
	if txRate > *maxTx {
		*maxTx = txRate
	}
	logger.Printf("当前上传速率: %s", formatBytes(txRate))
	return nil
}

// adjustSpeedLimit 调整速率限制
func adjustSpeedLimit(currentMax int64) {
	logger.Println("执行限速调整...")

	var newLimit int64
	if currentMax > appConfig.Threshold {
		newLimit = (appConfig.TotalBandwidth-currentMax)/8 - appConfig.RateLimitOffset
		if newLimit < 0 {
			newLimit = 0
		}
		logger.Printf("检测到外网流量，限制QB上传为 %s", formatBytes(newLimit))
	} else {
		newLimit = 0
		logger.Println("外网流量正常，解除QB上传限制")
	}

	if err := setUploadLimit(newLimit); err != nil {
		logger.Printf("限速设置失败: %v", err)
	}
}

// setUploadLimit 设置QB上传限速
func setUploadLimit(limit int64) error {
	data := url.Values{"limit": []string{strconv.FormatInt(limit, 10)}}
	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Cookie":       "SID=" + sid,
	}

	// 首次尝试设置
	if _, _, err := sendRequest(appConfig.QBittorrentURL+QBSetUploadLimitURL, data, headers); err != nil {
		// 会话过期时重新登录
		logger.Println("检测到会话过期，尝试重新登录...")
		if newSid, err := qbLogin(); err != nil {
			return fmt.Errorf("重新登录失败: %w", err)
		} else {
			sid = newSid
			headers["Cookie"] = "SID=" + sid
			_, _, err = sendRequest(appConfig.QBittorrentURL+QBSetUploadLimitURL, data, headers)
			return err
		}
	}
	return nil
}

// qbLogin QBittorrent登录认证
func qbLogin() (string, error) {
	data := url.Values{
		"username": []string{appConfig.Username},
		"password": []string{appConfig.Password},
	}

	body, headers, err := sendRequest(
		appConfig.QBittorrentURL+QBLoginURL,
		data,
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
	)

	if err != nil {
		logger.Printf("登录响应: %s", body)
		return "", fmt.Errorf("API请求失败: %w", err)
	}

	// 从Cookie提取会话ID
	for _, cookie := range strings.Split(headers["Set-Cookie"], ";") {
		if strings.HasPrefix(cookie, "SID=") {
			return strings.TrimPrefix(cookie, "SID="), nil
		}
	}
	return "", fmt.Errorf("会话ID获取失败")
}

// sendRequest 通用HTTP请求工具
func sendRequest(url string, data url.Values, headers map[string]string) ([]byte, map[string]string, error) {
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(data.Encode()))
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("请求发送失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("异常状态码: %d", resp.StatusCode)
	}

	// 处理响应数据
	body, _ := io.ReadAll(resp.Body)
	resHeaders := make(map[string]string)
	for k, v := range resp.Header {
		resHeaders[k] = v[0]
	}
	return body, resHeaders, nil
}

// getPidByName 通过进程名获取PID
func getPidByName() (string, error) {
	// 动态获取配置的进程名
	processName := appConfig.MonitorProcess
	cmd := fmt.Sprintf("ps aux | grep '%s' | grep -v grep | awk '{print $2}'", processName)

	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return "", fmt.Errorf("进程查询失败: %w", err)
	}
	pid := strings.TrimSpace(string(out))
	if pid == "" {
		return "", fmt.Errorf("进程未找到")
	}
	return pid, nil
}

// formatBytes 字节单位转换工具
func formatBytes(b int64) string {
	units := []string{"B", "KB", "MB", "GB"}
	v := float64(b)
	i := 0
	for ; i < len(units)-1 && v >= 1024; i++ {
		v /= 1024
	}
	return fmt.Sprintf("%.2f %s/s", v, units[i])
}
