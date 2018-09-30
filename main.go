package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stdout, "run: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	u := NewUniverse()
	go u.Run(context.Background())

	http.Handle("/", http.FileServer(http.Dir(".")))
	http.Handle("/ws", gameSocketHandler(u))

	if err := http.ListenAndServe(":5000", nil); err != nil {
		return fmt.Errorf("http: %s", err)
	}
	return nil
}

func gameSocketHandler(u *universe) http.HandlerFunc {
	var upgrader websocket.Upgrader

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, quit := context.WithCancel(r.Context())
		defer quit()

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer ws.Close()

		recv := make(chan []byte)
		send, id := u.AddPlayer(recv)
		defer u.DelPlayer(id)

		log.Println("client connected", id)
		defer log.Println("client disconnected", id)

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case b := <-send:
					if err := ws.WriteMessage(websocket.TextMessage, b); err != nil {
						quit()
						return
					}
				}
			}
		}()

		for {
			_, b, err := ws.ReadMessage()
			if err != nil {
				quit()
				return
			}
			select {
			case <-ctx.Done():
				return
			case recv <- b:
			}
		}

	}
}
