package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/sakurayung/gochat/internal/chat"
	"github.com/sakurayung/gochat/internal/transport"
)

func main() {
	log.SetFlags(0)

	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

// run initializes the chatServer and then
// starts a http.Server for the passed in address.
func run() error {
	if len(os.Args) < 2 {
		return errors.New("please provide an address to listen on as the first argument")
	}

	l, err := net.Listen("tcp", os.Args[1])
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	log.Printf("listening on ws://%v", l.Addr())

	// The wiring: transport owns I/O, chat owns room/rules
	srv := transport.NewServer(transport.Config{
		Rooms:  chat.NewRegistry(),
		Logger: logger,
		Assets: http.FileServer(http.Dir("web")),
	})

	// AnnounceLimiter + OutboundQueue left as zero values
	// so NewServer fills in its defaults.
	s := &http.Server{
		Handler:      srv,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}
	errc := make(chan error, 1)
	go func() {
		errc <- s.Serve(l)
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select {
	case err := <-errc:
		log.Printf("failed to serve: %v", err)
	case sig := <-sigs:
		log.Printf("terminating: %v", sig)
	}

	// shutdown order matters for websockets:
	// 1. DisconnectAll() closes every hijacked WS connection with GoingAway, which
	// unblocks each conn's 3 goroutines (writePump / pingLoop / readLoop in subscribe.go)
	//
	// 2. Shutdown() stops accepting + waits for in-flight HTTP (rooms / announce / static assets).
	//
	// http.Server.Shutdown alone can't do step 1 - it doesn't track hijacked connections.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	srv.DisconnectAll()

	return s.Shutdown(ctx)
}
