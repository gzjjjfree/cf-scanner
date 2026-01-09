package scanner

import (
	"context"
	"fmt"

	"github.com/eiannone/keyboard"
)

func ListenForStopKey(ctx context.Context, cancel context.CancelFunc, done chan struct{}) {
	if err := keyboard.Open(); err != nil {
		fmt.Println("无法初始化键盘监听:", err)
		close(done)
		return
	}
	defer func() {
		// 调用库函数（不带参数）
		_ = keyboard.Close()

		close(done)
	}()

	// 创建一个通道用于接收按键
    keyChan := make(chan struct {
        char rune
        key  keyboard.Key
        err  error
    })

    // 启动一个专门阻塞等待按键的协程
    go func() {
        for {
            char, key, err := keyboard.GetKey()
            keyChan <- struct {
                char rune
                key  keyboard.Key
                err  error
            }{char, key, err}
        }
    }()

	fmt.Println("\n[操作提示] 按 [Esc] 键可优雅停止扫描并保存当前结果，按 [Ctrl+C] 强制退出。")

	for {
        select {
        case <-ctx.Done():
            // 扫描结束信号：主程序通知退出
            return
        case k := <-keyChan:
            // 按键信号：收到了用户输入
            if k.err != nil {
                return
            }
            if k.key == keyboard.KeyEsc {
				StatusMutex.Lock()
				Status.WaitStop = true
				StatusMutex.Unlock()
                cancel() 
                return
            }
        }
    }
}
