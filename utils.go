package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
)

type IPItem struct {
	Address string `json:"address"`
}

// readLines 从文件中读取所有行
func readLines(path string) ([]string, bool, error) {
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

	// 检查是否为 IPv6 且掩码过大
	ones, bits := ipnet.Mask.Size()
	// 如果是 IPv6 且掩码小于 120 (剩下的 IP 太多)，则不能全量遍历
	if bits == 128 && ones < 120 {
		return handleLargeIPv6(ipnet), nil
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

// 专门处理大型 IPv6 段：随机抽取一定数量的 IP (已弃)
func handleLargeIPv6(ipnet *net.IPNet) []string {
	var ips []string
	targetCount := 512 // 随机抽取 512 个样本

	// 获取网段的起始 IP (16字节)
	baseIP := ipnet.IP
	// 获取掩码
	mask := ipnet.Mask

	rand.Seed(time.Now().UnixNano())

	for i := 0; i < targetCount; i++ {
		// 1. 创建一个新的 16 字节切片作为候选 IP
		randomIP := make(net.IP, 16)

		// 2. 生成 16 字节的随机数据
		rand.Read(randomIP)

		// 3. 核心位运算：
		// 结果 IP = (基础网段 IP & 掩码) | (随机数据 & ~掩码)
		// 简单来说：保留掩码覆盖的位，其余位填充随机数
		for j := 0; j < 16; j++ {
			// (baseIP[j] & mask[j]) 提取网络前缀部分
			// (randomIP[j] & ^mask[j]) 提取随机的主机部分
			randomIP[j] = (baseIP[j] & mask[j]) | (randomIP[j] & ^mask[j])
		}

		ips = append(ips, randomIP.String())
	}

	return ips
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
