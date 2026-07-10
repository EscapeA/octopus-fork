package main

import (
	_ "time/tzdata" // 内嵌 IANA 时区数据库，保证非 Docker 环境（无 tzdata）也能 time.LoadLocation

	"github.com/lingyuins/octopus/cmd"
)

func main() {
	cmd.Execute()
}
