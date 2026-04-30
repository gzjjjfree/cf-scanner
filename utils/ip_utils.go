package utils

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"

	"github.com/schollz/progressbar/v3"
)

type IPItem struct {
	Address string `json:"address"`
}

// Logger 定义了需要的日志能力
// 只要任何类型实现了这个 WriteLog 方法，就可以传参过来使用 WriteLog 方法
type Logger interface {
	WriteLog(string)
	GetTheme() progressbar.Theme
}

type UtilsLogger struct {
	Theme progressbar.Theme
	Ctx   context.Context
}

// 让 WebLogger 实现 WriteLog 方法
func (w UtilsLogger) WriteLog(msg string) {
	fmt.Println(msg)
}

func (w UtilsLogger) GetTheme() progressbar.Theme {
	return w.Theme
}

var BridgeLogger = UtilsLogger{
	Theme: progressbar.Theme{
		Saucer:        "=",
		SaucerHead:    ">",
		SaucerPadding: " ",
		BarStart:      "[",
		BarEnd:        "]",
	},
}

type writerWrapper struct {
	l Logger
}

func (w writerWrapper) Write(p []byte) (n int, err error) {
	// 将进度条发送过来的 []byte 转换为 string，并调用你的接口方法
	msg := string(p)

	w.l.WriteLog(msg)

	return len(p), nil
}

func ParseIP(ctx context.Context, ipFile string, testCount int, l Logger) ([][]string, int) {
	// --- 新增：读取黑名单 ---
	blacklist := readBlacklist("notwork.json")

	// 读取并解析 IP 段文件
	cidrList, isJSONInput, err := ReadLines(ipFile)
	if err != nil {
		l.WriteLog(fmt.Sprintln("读取 IP 文件出错, 正下载 IP 文件重新读取......！"))
		derr := DownloadFile(ctx, "https://www.cloudflare.com/ips-v4", "ip.txt", BridgeLogger)
		if derr != nil {
			l.WriteLog(fmt.Sprintf("无法下载 IP 文件: %v\n请多尝试几次或自行下载: https://www.cloudflare.com/ips-v4", derr))
			return nil, 0
		}
		cidrList, isJSONInput, err = ReadLines("ip.txt")
		if err != nil {
			l.WriteLog(fmt.Sprintf("无法读取 IP 文件: %v\n请把合适的 IP 文件放在根目录下\n", err))
			return nil, 0
		}
	}

	// 每段分别取样
	ipGroups := make([][]string, 1)	

	for _, cidr := range cidrList {
		ips, _ := ParseCIDR(cidr)

		// --- 过滤黑名单中的 IP ---
		var filteredIps []string
		for _, ip := range ips {
			if _, found := blacklist[ip]; !found {
				filteredIps = append(filteredIps, ip)
			}
		}

		// 如果该段过滤后没有剩余 IP，直接跳过
		if len(filteredIps) == 0 {
			continue
		}

		if isJSONInput {
			// json 文件全部 ip 读入groups[0]
			ipGroups[0] = append(ipGroups[0], filteredIps...)
			// l.WriteLog(fmt.Sprintf("JSON 输入: IP 段 [%v] 过滤后剩余 %v 个 IP (已剔除黑名单)\n", cidr, len(filteredIps)))
		} else {
			// 每个 ip 段分别取样
			groups := pickSamples(ips, testCount)
			l.WriteLog(fmt.Sprintf("IP 段 [%v] 随机抽样数为: %v (已剔除黑名单)\n", cidr, len(groups)))
			// 二维切片 ipGroups 的每个切片都是一个 ip 段取样的结果
			ipGroups = append(ipGroups, groups)
		}
	}

	// 预计算总数
	actualTaskCount := 0
	for i := 0; i < len(ipGroups); i++ {
		for o := 0; o < len(ipGroups[i]); o++ {
			actualTaskCount++
		}
	}

	logMsg := fmt.Sprintf("解析完成，总计 %d 个 IP，开始随机抽样扫描...\n", actualTaskCount)
	l.WriteLog(logMsg)
	return ipGroups, actualTaskCount
}

// readBlacklist 从指定的 JSON 文件中读取黑名单 IP，并返回一个包含这些 IP 的 map
func readBlacklist(filename string) map[string]struct{} {
	blacklist := make(map[string]struct{})
	data, err := os.ReadFile(filename)
	if err != nil {
		return blacklist // 如果文件不存在或读取失败，返回空 map
	}

	// 定义匹配 JSON 的临时结构体
	var entries []struct {
		Address string `json:"address"`
	}

	if err := json.Unmarshal(data, &entries); err == nil {
		for _, entry := range entries {
			blacklist[entry.Address] = struct{}{}
		}
	}
	return blacklist
}

// readLines 从文件中读取所有行
func ReadLines(path string) ([]string, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}

	trimmed := strings.TrimSpace(string(content))
	isJSON := strings.HasPrefix(trimmed, "[")
	var ips []string

	// --- 逻辑判断：如果是 JSON 格式 ---
	if strings.HasPrefix(trimmed, "[") {
		var items []IPItem
		if err := json.Unmarshal(content, &items); err == nil {
			for _, item := range items {
				if item.Address != "" {
					ips = append(ips, item.Address)
				}
			}
			return ips, isJSON, nil
		}
	}

	// --- 逻辑判断：如果是普通文本格式 ---
	scanner := bufio.NewScanner(strings.NewReader(trimmed))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			ips = append(ips, line)
		}
	}
	return ips, false, scanner.Err()
}

// ParseCIDR 将网段（如 1.1.1.0/24）解析为具体的 IP 列表
func ParseCIDR(cidr string) ([]string, error) {
	if !strings.Contains(cidr, "/") {
		trialIP := net.ParseIP(cidr)
		if trialIP != nil {
			return []string{cidr}, nil
		}
		return nil, fmt.Errorf("无效格式")
	}

	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	// 常规遍历 (适用于 IPv4 或极小的 IPv6 段)
	var ips []string
	for curr := ip.Mask(ipnet.Mask); ipnet.Contains(curr); inc(curr) {
		// 注意：net.IP 是切片，必须复制一份，否则 append 的全是同一个值
		temp := make(net.IP, len(curr))
		copy(temp, curr)
		ips = append(ips, temp.String())
	}

	if len(ips) <= 2 {
		return ips, nil
	}
	return ips[1 : len(ips)-1], nil
}

// 通用的 IP 自增函数，支持 IPv4 和 IPv6
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// ip 段取样
func pickSamples(ips []string, testCount int) []string {
	// 引入随机步长
	targetCount := testCount // 我们希望最终测试的 IP 数量
	var currentStep int

	totalIPs := len(ips)
	if totalIPs <= targetCount {
		// 如果 IP 总数还没到希望最终测试的数量，没必要抽样，直接全测
		currentStep = 1
	} else {
		// 自动计算步长：总数 / 目标数
		// 例如：500,000 / 200 = 2500 (步长)
		currentStep = totalIPs / targetCount
	}

	var sampled []string

	for i := 0; i < totalIPs; i += currentStep {
		// 计算当前区间的结束位置
		end := i + currentStep
		if end > totalIPs {
			end = totalIPs
		}

		// 在 [i, end) 区间内随机选一个索引
		randomIndex := i + rand.Intn(end-i)
		sampled = append(sampled, ips[randomIndex])
	}

	return sampled
}
