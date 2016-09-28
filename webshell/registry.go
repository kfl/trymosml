package main

// registry maintains the set of active clients

type Registry struct {
	// Registered clients.
	clients map[*Client]bool

	// Register a client.
	register chan *Client

	// Unregister a client.
	unregister chan *Client
}

func newRegistry() *Registry {
	return &Registry{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (r *Registry) run() {
	for {
		select {
		case client := <-r.register:
			r.clients[client] = true
		case client := <-r.unregister:
			if _, ok := r.clients[client]; ok {
				delete(r.clients, client)
				//close(client.send)
			}
		}
	}
}
