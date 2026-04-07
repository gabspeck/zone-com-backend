package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"zone.com/internal/server"
)

func main() {
	port := flag.Int("port", 28805, "listen port")
	tables := flag.Int("tables", 100, "number of tables")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	log.Printf("========================================")
	log.Printf("Zone.com Checkers Server starting")
	log.Printf("  port:   %d", *port)
	log.Printf("  tables: %d", *tables)
	log.Printf("  seats:  2 per table")
	log.Printf("========================================")

	srv := server.New(*port, *tables)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := srv.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
