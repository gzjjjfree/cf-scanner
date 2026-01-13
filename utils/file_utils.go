package utils

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gzjjjfree/cf-scanner/scanner"
	"github.com/schollz/progressbar/v3"
)

// saveToCSV 保存详细报告
func SaveToCSV(filename string, data []scanner.FinalResult) {
	// 1. 提取目录路径并创建 (例如 "result/result.csv")
	dir := filepath.Dir(filename)

	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Println("创建目录失败:", err)
		return
	}

	// 2. 创建文件
	file, err := os.Create(filename)
	if err != nil {
		log.Println("创建文件失败:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"IP 地址", "延迟", "下载速度", "时间"})
	for _, r := range data {
		writer.Write([]string{
			r.IP,
			r.Latency,
			fmt.Sprintf("%.2f", r.DownloadMBs),
			r.CreatedAt.Format("2006-01-02 15:04:05"), // Go 的标准时间格式化写法
		})
	}
}

// saveToJSON 仅保存地址列表
func SaveToJSON(filename string, data []scanner.FinalResult) {
	file, _ := os.Create(filename)
	defer file.Close()

	// 只需要 JSON 里显示 address 字段，
	// FinalResult 里的其他字段在定义时加了 omitempty，且没有赋值时就会被隐藏
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	encoder.Encode(data)
}

// 追加保存到指定 JSON 文件
func AppendToJSONFile(path string, newResults []scanner.FinalResult) error {
	var existingData []map[string]interface{}

	// 尝试读取现有文件
	fileData, err := os.ReadFile(path)
	if err == nil && len(fileData) > 0 {
		// 如果文件存在且不为空，解析现有内容
		if err := json.Unmarshal(fileData, &existingData); err != nil {
			// 如果解析失败，说明原文件可能不是合法的 JSON 数组，记录警告
			fmt.Printf("警告: 原文件格式不兼容，将创建新数组: %v\n", err)
			existingData = []map[string]interface{}{}
		}
	}

	// 将新结果转换为 map 结构（为了只保留带 json 标签的字段）
	// 这样做可以确保忽略那些标记为 `json:"-"` 的字段
	for _, res := range newResults {
		// 我们通过这种方式只提取带 json 标签的字段
		item := map[string]interface{}{
			"address": res.IP,
		}

		// 可选：在这里做去重逻辑
		isDuplicate := false
		for _, existing := range existingData {
			if existing["address"] == res.IP {
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			existingData = append(existingData, item)
		}
	}

	// 序列化回 JSON 数组（带缩进方便阅读）
	updatedJSON, err := json.MarshalIndent(existingData, "", "    ")
	if err != nil {
		return err
	}

	// 覆盖写入文件
	return os.WriteFile(path, updatedJSON, 0644)
}

// 从 CF 官网下载 IP 列表并保存
func DownloadFile(ctx context.Context, url string, filename string, l Logger) error {
	// 1. 创建带有 Context 的请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// 如果是因为取消导致的错误，直接返回
		if errors.Is(err, context.Canceled) {
			l.WriteLog("下载已取消")
			return err
		}
		return fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回状态码异常: %d", resp.StatusCode)
	}

	// 2. 创建目标文件
	out, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer out.Close()

	loggerAdapter := writerWrapper{l: l}
	l.WriteLog(fmt.Sprintf("📂 目标路径: %s\n", filename))
	bar := progressbar.NewOptions64(
		resp.ContentLength,
		progressbar.OptionSetDescription("下载进度:"),
		progressbar.OptionSetWriter(loggerAdapter),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowTotalBytes(true),
		progressbar.OptionSetWidth(20),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		//progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
	)
	defer bar.Close()

	// io.Copy 默认不可中断，我们需要手动在循环中检查 Context 状态
	_, err = copyWithContext(ctx, io.MultiWriter(out, bar), resp.Body)
	bar.Finish()
	
	if err != nil {
		if errors.Is(err, context.Canceled) {
			l.WriteLog("\n下载任务已手动停止\n")
			// 任务停止后，删除未下载完的残留文件
			out.Close()
			os.Remove(filename)
			return err
		}
		return err
	}

	l.WriteLog(fmt.Sprintln("\n下载完成:", filename))
	return nil
}

// 辅助函数：支持 Context 的拷贝
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	// 每次拷贝 32KB 数据就检查一次是否取消
	buf := make([]byte, 32*1024)
	var written int64
	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
			nr, er := src.Read(buf)
			if nr > 0 {
				nw, ew := dst.Write(buf[0:nr])
				if nw > 0 {
					written += int64(nw)
				}
				if ew != nil {
					return written, ew
				}
			}
			if er != nil {
				if er == io.EOF {
					return written, nil
				}
				return written, er
			}
		}
	}
}

func GetDownloadURL(baseURL string, version string, repoName string) string {
	//baseURL := "https://github.com/gzjjjfree/v5-result/releases/download"
	//https://github.com/gzjjjfree/v5-result/releases/download/custom-build/v5-result-windows-amd64.exe
	var osName, arch, extension string

	// 判定操作系统
	switch runtime.GOOS {
	case "windows":
		osName = "windows"
		extension = ".exe"
	case "darwin": // macOS
		osName = "macos"
		extension = ""
	case "linux":
		osName = "linux"
		extension = ""
	default:
		osName = "linux"
		extension = ""
	}

	// 判定架构
	switch runtime.GOARCH {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		arch = "amd64"
	}

	// 拼接符合你 Release 命名规则的字符串
	// 假设你的命名是：v5-result-windows-64.zip
	fileName := fmt.Sprintf("%s-%s-%s%s", repoName, osName, arch, extension)
	return fmt.Sprintf("%s/%s/%s", baseURL, version, fileName)
}

var DownloadMsg = `
这是基于 v5-result 分支自动生成的定制版 V2Ray。
已注入 cf-scanner 的 result.json 自动加载逻辑。
请把 result.json 放在 v5-result 同目录的 result 文件夹里
json 格式为：
[
  {
    "address": "104.16.244.51"
  },
  {
    "address": "104.19.46.94"
  }
]
config.json 文件配置的出站 tag 只要头部为 “cdn-” 如 "cdn-proxy"
则生成由 result.json 地址池 IP 组成的出站列表，tag 为 "cdn-proxy-序号"
出站路由写: 
"outbounds": [
  {
    "protocol": "vless",
    "tag": "cdn-vless",
    "settings": {
      "vnext": [
        {
          "address" //可不填, 填了也会被覆盖, 生成除 address 不同, 其他配置相同的出站列表
          "port": 443,
          "users": [
            {
              "id": "UUID",
              "encryption": "none"
            }
          ]
        }
      ]
    },
    "streamSettings": {
      "network": "ws",
      "security": "tls",
      "tlsSettings": {
        "serverName": "cf 代理你的网站名",
        "allowInsecure": false
      },
      "wsSettings": {
        "path": "/",
        "headers": {
          "Host": "cf 代理你的网站名"
        }
      }
    }
  }
]
"routing": {
  "domainStrategy": "AsIs",
  "balancers": [
    {
      "tag": "cdn-balancer",
      "selector": [
        "cdn-"
      ],
      "strategy": {
        "type": "random"
      }
    }
  ],
  "rules": [
    {
      "type": "field",
      "balancerTag": "cdn-balancer",
      "inboundTag": "yourfrom"
    }
  ]
}`
