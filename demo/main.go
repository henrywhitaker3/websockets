package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/henrywhitaker3/websockets"
)

type handler struct{}

func (h handler) Empty() any {
	var out string
	return out
}

func (h handler) Handle(conn websockets.Connection, content any) error {
	fmt.Printf("%s: %s\n", time.Now().Format(time.RFC3339), content.(string))
	return nil
}

const (
	Demo websockets.Topic = "demo"
)

func main() {
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	server := websockets.NewServer(websockets.ServerOpts{
		Logger: websockets.NewSlog(log),
	})
	if err := server.Register(Demo, handler{}); err != nil {
		log.Error("failed to register server handler", "error", err)
		os.Exit(1)
	}
	go http.ListenAndServe(":17456", server)

	client, err := websockets.NewClient(websockets.ClientOpts{
		Addr:   "ws://localhost:17456",
		Logger: websockets.NewSlog(log),
	})
	if err != nil {
		log.Error("failed to connect", "error", err)
		os.Exit(1)
	}
	if err := client.Register(Demo, handler{}); err != nil {
		log.Error("failed to register client handler", "error", err)
		os.Exit(1)
	}

	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if err := client.Send(ctx, Demo, "message from client"); err != nil {
					log.Error("client failed to send message", "error", err)
				}
			}
		}
	}()

	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if err := server.Broadcast(ctx, Demo, "broadcast from server"); err != nil {
					log.Error("server failed to broadcast", "error", err)
				}
			}
		}
	}()

	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if err := server.Once(ctx, Demo, "once from server"); err != nil {
					log.Error("server failed to once", "error", err)
				}
			}
		}
	}()

	<-ctx.Done()
}
