package scanner

import (
	"context"
	"fmt"

	"github.com/eiannone/keyboard"
)

func ListenForStopKey(ctx context.Context, cancel context.CancelFunc) {
	if err := keyboard.Open(); err != nil {
		fmt.Println("无法初始化键盘监听:", err)
		return
	}
	defer keyboard.Close()

	fmt.Println("\n[操作提示] 按 [Esc] 键可优雅停止扫描并保存当前结果，按 [Ctrl+C] 强制退出。")

	// 定义旋转字符
	var spinnerChars = []string{"\\", "|", "/", "-"}

	ctxSpinner, cancelSpinner := context.WithCancel(ctx)
	go startSpinner(ctxSpinner, spinnerChars) // 启动旋转图标
	defer cancelSpinner()

	for {
		// GetKey 会阻塞，直到有按键按下
		_, key, err := keyboard.GetKey()
		if err != nil {
			break
		}

		// 判定 Esc 键
		if key == keyboard.KeyEsc {
			cancel() // 触发 Context 取消，所有扫描协程会收到信号
			cancelSpinner()
			break
		}
	}
}
