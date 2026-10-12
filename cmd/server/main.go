package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/PaNasMs/module-file-drop/internal/drop"
	"github.com/PaNasMs/module-sdk/modulehost"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "write" {
		if len(os.Args) != 5 {
			log.Fatal("invalid writer arguments")
		}
		size, err := strconv.ParseInt(os.Args[4], 10, 64)
		if err != nil {
			log.Fatal("invalid file size")
		}
		if err = drop.WriteFromStdin(os.Args[2], os.Args[3], size); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	var service *drop.Service
	modulehost.ServeWithActivity("file-drop", func() int32 { return service.Active() }, func(allowed map[string]bool) http.Handler {
		service = drop.New("/var/lib/panasms-file-drop", allowed)
		return service.Handler(allowed)
	})
}
