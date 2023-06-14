package main

import (
	"fmt"

	"cli/cmd/root"
	"cli/cmd/config"
)

func main() {
    config.InitConfig()
 
    root.NewRootCommand().Execute()
 
    fmt.Println("")
}
