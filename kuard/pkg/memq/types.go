package memq

import "time"

type Stats struct {
	Kind   string `json:"kind"`
	Queues []Stat `json:"queues"`
}

type Stat struct {
	Name     string `json:"name"`
	Depth    int64  `json:"depth"`
	Enqueued int64  `json:"enqueued"`
	Dequeued int64  `json:"dequeued"`
	Drained  int64  `json:"drained"`
}

type Message struct {
	Kind    string    `json:"kind"`
	ID      string    `json:"id"`
	Body    string    `json:"body"`
	Created time.Time `json:"creationTimestamp"`
}
