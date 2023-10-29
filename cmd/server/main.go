package main

import (
	"log"
	"proxy-server/internal/pkg/service"
)

func main() {
	server := service.NewServer(":3000")
	log.Fatal(server.Start())
}
