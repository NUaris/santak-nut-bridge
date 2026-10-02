package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var version = "0.1.0"

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	if cfg.ShowVersion {
		fmt.Println(version)
		return
	}
	if cfg.Once {
		snapshot, queryErr := queryOnce(cfg)
		if queryErr != nil {
			log.Fatal(queryErr)
		}
		output, marshalErr := json.MarshalIndent(snapshot, "", "  ")
		if marshalErr != nil {
			log.Fatal(marshalErr)
		}
		fmt.Println(string(output))
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	store := &SnapshotStore{}
	poller := &SerialPoller{config: cfg, store: store}
	go poller.Run(ctx)
	server := &NUTServer{config: cfg, store: store}
	if err := server.ListenAndServe(ctx); err != nil {
		log.Fatal(err)
	}
}
