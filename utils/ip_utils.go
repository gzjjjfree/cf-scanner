package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
)

type IPItem struct {
	Address string `json:"address"`
}

func ParseIP(c Config) ([][]string, int) {
	// 读取并解析 IP 段文件
	cidrList, isJSONInput, err := ReadLines(c.IPFile)
	if err != nil {
		fmt.Printf("无法读取 IP 文件: %v\n", err)
		return nil, 0
	}

	// 每段分别取样
	ipGroups := make([][]string, 1)
	for _, cidr := range cidrList {
		ips, _ := ParseCIDR(cidr)
		if isJSONInput {
			// json 文件全部 ip 读入groups[0]
			ipGroups[0] = append(ipGroups[0], ips...)
		} else {
			// 每个 ip 段分别取样
			groups := pickSamples(ips, c.TestCount)
			fmt.Printf("IP 段 [%v] 随机抽样数为: %v\n", cidr, len(groups))
			// 二维切片 ipGroups 的每个切片都是一个 ip 段取样的结果
			ipGroups = append(ipGroups, groups)
		}
	}

	// 预计算总数 (非常重要！)
	actualTaskCount := 0
	for i := 0; i < len(ipGroups); i++ {
		for o := 0; o < len(ipGroups[i]); o++ {
			actualTaskCount++
		}
	}

	fmt.Printf("解析完成，总计 %d 个 IP，开始随机抽样扫描...\n", actualTaskCount)
	return ipGroups, actualTaskCount
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
		// 如果 JSON 解析失败，则尝试按普通文本处理（回退机制）
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
