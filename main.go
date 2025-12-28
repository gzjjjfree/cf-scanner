package main

import (
	"runtime"

	"github.com/gzjjjfree/cf-scanner/cmd"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	cmd.Execute()
}
