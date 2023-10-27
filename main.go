package main

import (
	"log"
	"os"

	"github.com/ehgks0000/rf-server-go/api"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func main() {

	pid := os.Getpid()
	log.Println("pid :", pid)

	envFile := ".env"
	if len(os.Args) > 1 {
		envFile = os.Args[1]
	}

	err := godotenv.Load(envFile)
	if err != nil {
		log.Fatalf("Error loading %s file", envFile)
	}

	port := os.Getenv("PORT")

	server, err := api.NewServer()

	if err != nil {
		log.Println("main err :", err)
	}

	// server.router.Run(port)
	server.Run(port)
}
