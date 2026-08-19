package main

import (
	"fmt"
	"os"
)

func greeting(who string) string {
	if who == "" {
		who = "XCODESPACES"
	}
	return "olá, " + who
}

func main() {
	who := os.Getenv("TESTE_WHO")
	fmt.Println(greeting(who))
}
