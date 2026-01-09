package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"runtime"

	"github.com/zserge/lorca"
)

//go:embed all:frontend/dist/*
var fsa embed.FS

var ui lorca.UI

func main() {
	// 1. 检查 Chrome 是否安装，如果没有，提示用户下载
	if runtime.GOOS == "windows" {
		// Lorca 在 Windows 上会自动寻找 Chrome/Edge
	}

	// 1. 获取子文件系统
	subFS, err := fs.Sub(fsa, "frontend/dist")
	if err != nil {
		fmt.Printf("读取嵌入文件失败: %v", err)
		return
	}

	// 1. 设置网络服务
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go http.Serve(ln, http.FileServer(http.FS(subFS)))

	args := []string{
		"--window-size=1280,900",
		"--window-position=360,40",
		"--disable-background-networking",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-breakpad",
		"--disable-client-side-phishing-detection",
		"--disable-default-apps",
		"--disable-dev-shm-usage",
		"--disable-infobars",
		"--disable-extensions",
		"--remote-allow-origins=*", // 关键：允许所有来源的远程调试连接
	}
	
	// 2. 启动 Lorca
	ui, err = lorca.New(fmt.Sprintf("http://%s/", ln.Addr()), "", 960, 600, args...)
	if err != nil {
		fmt.Printf("启动失败，请检查是否安装 Chrome/Edge: %v\n", err)
		return
	}
	defer ui.Close()

	// 注册函数
	registerFuns()

	<-ui.Done()
}
