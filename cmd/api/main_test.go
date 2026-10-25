package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestGracefulShutdownDrainsRequests(t *testing.T) {
	// Réserve une adresse locale disponible avant de la transmettre au serveur.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	entered := make(chan struct{})
	release := make(chan struct{})
	server := &http.Server{Addr: addr, ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		_, _ = w.Write([]byte("terminé"))
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- serve(ctx, server, 2*time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil))) }()
	requestDone := make(chan error, 1)
	go func() {
		client := http.Client{Timeout: 3 * time.Second}
		var resp *http.Response
		var e error
		for n := 0; n < 100; n++ {
			resp, e = client.Get("http://" + addr)
			if e == nil {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if e == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if string(body) != "terminé" {
				e = io.ErrUnexpectedEOF
			}
		}
		requestDone <- e
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("le serveur n’a pas accepté la requête")
	}
	cancel()
	select {
	case e := <-finished:
		t.Fatalf("l’arrêt n’a pas attendu la requête en cours : %v", e)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if e := <-requestDone; e != nil {
		t.Fatal(e)
	}
	select {
	case e := <-finished:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("délai d’arrêt dépassé")
	}
}
