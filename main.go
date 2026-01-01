package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/gzjjjfree/cf-scanner/cmd"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if len(os.Args) == 1 {
		fmt.Println("\n[提示] 终端已就绪。")
		fmt.Println("请输入命令： cf-scanner.exe -run (开始默认扫描)")
		fmt.Println("或者输入： cf-scanner.exe -run -t 100 (指定 100 线程扫描)\n")

		executable, _ := os.Executable()

		cmd := exec.Command("cmd", "/S", "/K", executable, "-h")

		// 将新进程的输入输出直接挂载到当前创建的窗口
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		err := cmd.Run()
		if err != nil {
			fmt.Println("启动终端失败:", err)
		}
		return // 退出父进程，让子进程（CMD）接管
	}

	cmd.Execute()
}
