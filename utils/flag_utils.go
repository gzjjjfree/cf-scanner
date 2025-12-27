package utils

import (
	"flag"
)

// 辅助结构体，用于在包之间传递参数
type Config struct {
	Domain         string
	IPFile         string
	OutFile        string
	WorkerCount    int
	LatencyLimit   int64
	MinSpeed       float64
	OutCount       int
	TestCount      int
	AppendMode     bool
	OutputFilePath string
	ShowVersion    bool
	Help           bool
}

func ParseConfig() Config {
	c := Config{}

	// . 定义命令行参数
	flag.StringVar(&c.Domain, "d", "speed.cloudflare.com/__down?bytes=100000000", "SNI Domain")
	flag.StringVar(&c.IPFile, "f", "ip.txt", "包含 IP 段的文件路径")
	flag.StringVar(&c.OutFile, "o", "result", "输出文件路径加前缀 (不带后缀)")
	flag.IntVar(&c.WorkerCount, "n", 100, "并发协程数")
	flag.Int64Var(&c.LatencyLimit, "l", 200, "最低延时")
	flag.Float64Var(&c.MinSpeed, "s", 10, "最低下载")
	flag.IntVar(&c.OutCount, "on", 100, "最终结果数")
	flag.IntVar(&c.TestCount, "tn", 500, "单个 IP 段期望测试的 IP 数量")
	flag.BoolVar(&c.AppendMode, "a", false, "是否使用追加模式写入文件")
	flag.StringVar(&c.OutputFilePath, "p", "./okresult.json", "输出到指定 JSON 文件（追加模式）")
	flag.BoolVar(&c.ShowVersion, "v", false, "显示版本号")
	flag.BoolVar(&c.Help, "h", false, "显示帮助信息")

	flag.Parse()
	return c
}
