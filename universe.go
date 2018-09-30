package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

func NewUniverse() *universe {
	return &universe{
		size:    vec2{X: 100, Y: 100},
		players: make(map[uint16]*player),
		nextid:  1,
	}
}

type universe struct {
	size vec2

	sync.RWMutex

	players map[uint16]*player
	nextid  uint16
}

type player struct {
	id   uint16
	move vec2
	head *snakechunk
	dead bool

	send chan []byte
}

type snakechunk struct {
	pos  vec2
	next *snakechunk
}

func (u *universe) AddPlayer(recv <-chan []byte) (send <-chan []byte, id uint16) {
	u.Lock()
	u.nextid++
	p := &player{
		id:   u.nextid,
		dead: false,
		move: vec2{X: 1, Y: 0},
		head: &snakechunk{
			pos: vec2{X: 5, Y: 5},
			next: &snakechunk{
				pos: vec2{X: 4, Y: 5},
				next: &snakechunk{
					pos:  vec2{X: 3, Y: 5},
					next: nil,
				},
			},
		},
		send: make(chan []byte),
	}
	u.players[u.nextid] = p
	u.Unlock()

	go func() {
		for rawmsg := range recv {
			msg := make([]int, 0, 8)
			if err := json.Unmarshal(rawmsg, &msg); err != nil {
				continue
			}
			switch msg[0] {
			default:
				log.Printf("unknown message: %+v", msg)
			case 1:
				log.Printf("echo: %+v", msg)
				select {
				case p.send <- rawmsg:
				default:
				}
			case 2:
				if len(msg) != 2 {
					continue
				}
				switch msg[1] {
				case 1:
					p.move = vec2{-1, 0}
				case 2:
					p.move = vec2{1, 0}
				case 3:
					p.move = vec2{0, 1}
				case 4:
					p.move = vec2{0, -1}
				}
			}
		}
	}()

	// TODO: introduce connection message
	// u.broadcast([]byte("player connected"))

	return p.send, p.id
}

func (u *universe) DelPlayer(id uint16) {
	u.Lock()
	if _, ok := u.players[id]; ok {
		delete(u.players, id)
	}
	u.Unlock()

	// TODO: introduce disconnection message
	// u.broadcast([]byte("player disconnected"))
}

func (u *universe) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Second / 6)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := u.tick(); err != nil {
				return fmt.Errorf("tick universe: %s", err)
			}
			var b bytes.Buffer
			fmt.Fprint(&b, "[2,")
			if err := u.serializeTo(&b); err != nil {
				return fmt.Errorf("serialize universe: %s", err)
			}
			fmt.Fprint(&b, "]")
			u.broadcast(b.Bytes())
		}
	}
}

func (u *universe) broadcast(payload []byte) {
	u.RLock()
	defer u.RUnlock()

	// Broadcast state to all playerd, but ignore the slow ones.
	for _, p := range u.players {
		select {
		case p.send <- payload:
		default:
		}
	}
}

func (u *universe) tick() error {
	u.RLock()
	defer u.RUnlock()

	// Move all players.
	for _, player := range u.players {
		if player.dead {
			continue
		}
		removeLastChunk(player.head)
		player.head = &snakechunk{
			pos:  player.head.pos.Sum(player.move),
			next: player.head,
		}
	}

	// Check for going outside of the map.
	for _, player := range u.players {
		if player.dead {
			continue
		}
		if player.head.pos.X < 0 || player.head.pos.X >= u.size.X || player.head.pos.Y < 0 || player.head.pos.Y >= u.size.Y {
			player.dead = true
		}
	}

	// For every snake, check if it is not eating itself.
	for _, p := range u.players {
		for c := p.head.next; c != nil; c = c.next {
			// It is enough to check only collision with the head,
			// because it's the only segment that moved this turn.
			if p.head.pos.Eq(c.pos) {
				p.dead = true
			}
		}
	}

	// Check for collision of snakes with one another. Not optimal loop and
	// checks twice, but it is good enough for now.
	// Dead snakes are not moving, but they are colliding.
	for _, a := range u.players {
		for _, b := range u.players {
			if a.id == b.id {
				continue
			}

		checkTwoSnakeCollision:
			for aa := a.head; aa != nil; aa = aa.next {
				for bb := b.head; bb != nil; bb = bb.next {
					if aa.pos.Eq(bb.pos) {
						a.dead = true
						b.dead = true
						break checkTwoSnakeCollision
					}
				}
			}
		}
	}

	return nil
}

func removeLastChunk(s *snakechunk) {
	for {
		if s == nil || s.next == nil {
			return
		}
		if s.next.next == nil {
			// Remove last item.
			s.next = nil
			return
		}
		s = s.next
	}
}

func (u *universe) serializeTo(b io.Writer) error {
	u.RLock()
	payload := make([][]uint16, 0, len(u.players))
	for _, p := range u.players {
		snakeLen := 0
		for c := p.head; c != nil; c = c.next {
			snakeLen++
		}
		const headerLen = 2
		raw := make([]uint16, headerLen, snakeLen*2+headerLen)
		raw[0] = p.id
		if p.dead {
			raw[1] = 1
		}
		for c := p.head; c != nil; c = c.next {
			raw = append(raw, uint16(c.pos.X), uint16(c.pos.Y))
		}

		payload = append(payload, raw)
	}
	u.RUnlock()

	return json.NewEncoder(b).Encode(payload)
}
