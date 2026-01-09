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
		fmt.Println("请输入命令： ./cf-scanner.exe --run (开始默认扫描)")
		fmt.Print("或者输入： ./cf-scanner.exe -h 查看帮助信息\n\n")

		// 判断操作系统
		if runtime.GOOS == "windows" {
			executable, _ := os.Executable()
			var windowsCmd *exec.Cmd
			// Windows 环境：使用 cmd /K 保持窗口开启
			windowsCmd = exec.Command("cmd", "/S", "/K", executable, "-h")

			// 将新进程的输入输出直接挂载到当前创建的窗口
			windowsCmd.Stdout = os.Stdout
			windowsCmd.Stderr = os.Stderr
			windowsCmd.Stdin = os.Stdin

			err := windowsCmd.Run()
			if err != nil {
				fmt.Println("启动终端失败:", err)
			}
			return // 退出父进程，让子进程（CMD）接管
		}
	}

	cmd.Execute()
}
