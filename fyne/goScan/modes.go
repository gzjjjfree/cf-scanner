package goScan

import (
	"context"

	"github.com/gzjjjfree/cf-scanner/scanner"
	"github.com/schollz/progressbar/v3"
)

var (
	conf       scanner.ScanConfig
	LogChan    = make(chan string, 100)
	FinishChan = make(chan bool)
	CanRead  bool
	CanDownload bool
)

// 定义一个空的结构体，作为接口的载体
type FyneLogger struct {
	Theme progressbar.Theme
	Ctx   context.Context
}

func (w FyneLogger) WriteLog(msg string) {
	LogChan <- msg
}

func (w FyneLogger) GetTheme() progressbar.Theme {
	return w.Theme
}

func (w FyneLogger) GetColorCodes() bool {
	return false
}

var BridgeLogger = FyneLogger{
	Theme: progressbar.Theme{
		Saucer:        "=",
		SaucerHead:    ">",
		SaucerPadding: " ",
		BarStart:      "[",
		BarEnd:        "]",
	},
}

var ReadDownload = `	下载文件为 V2ray V5.42.0 定制增加了自动生成出站列表的功能。
	默认下载路径为 cf-scanner 同路径, cf-scanner 刚好生成同目录文件夹 result/result.json
	v5-result 会依次读取 result 文件夹里的 result*.json 加载
	config.json 文件配置的出站 tag 只要头部为 “cdn-” 如 "cdn-proxy"
	则生成由 result.json 地址池 IP 组成的同配置的出站列表, tag 为 "cdn-proxy-序号"
	具体说明请查看 v5-result 项目的 release 下载说明
    点击 <开始下载> 开始下载文件`
