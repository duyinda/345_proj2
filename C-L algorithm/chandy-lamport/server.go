package chandy_lamport

import "log"

// The main participant of the distributed snapshot protocol.
// Servers exchange token messages and marker messages among each other.
// Token messages represent the transfer of tokens from one server to another.
// Marker messages represent the progress of the snapshot process. The bulk of
// the distributed protocol is implemented in `HandlePacket` and `StartSnapshot`.
type Server struct {
	Id            string
	Tokens        int
	sim           *Simulator
	outboundLinks map[string]*Link // key = link.dest
	inboundLinks  map[string]*Link // key = link.src
	// TODO: ADD MORE FIELDS HERE
	receivedSnapshot map[int]bool            // snapshotID -> if received snapshot
	inReceivedMarker map[int]map[string]bool // snapshotID -> src -> if received marker
	snapshot         map[int]*SnapshotState  // snapshotID -> state
}

// A unidirectional communication channel between two servers
// Each link contains an event queue (as opposed to a packet queue)
type Link struct {
	src    string
	dest   string
	events *Queue
}

func NewServer(id string, tokens int, sim *Simulator) *Server {
	return &Server{
		id,
		tokens,
		sim,
		make(map[string]*Link),
		make(map[string]*Link),
		make(map[int]bool),
		make(map[int]map[string]bool),
		make(map[int]*SnapshotState),
	}
}

// Add a unidirectional link to the destination server
func (server *Server) AddOutboundLink(dest *Server) {
	if server == dest {
		return
	}
	l := Link{server.Id, dest.Id, NewQueue()}
	server.outboundLinks[dest.Id] = &l
	dest.inboundLinks[server.Id] = &l
}

// Send a message on all of the server's outbound links
func (server *Server) SendToNeighbors(message interface{}) {
	for _, serverId := range getSortedKeys(server.outboundLinks) {
		link := server.outboundLinks[serverId]
		server.sim.logger.RecordEvent(
			server,
			SentMessageEvent{server.Id, link.dest, message})
		link.events.Push(SendMessageEvent{
			server.Id,
			link.dest,
			message,
			server.sim.GetReceiveTime()})
	}
}

// Send a number of tokens to a neighbor attached to this server
func (server *Server) SendTokens(numTokens int, dest string) {
	if server.Tokens < numTokens {
		log.Fatalf("Server %v attempted to send %v tokens when it only has %v\n",
			server.Id, numTokens, server.Tokens)
	}
	message := TokenMessage{numTokens}
	server.sim.logger.RecordEvent(server, SentMessageEvent{server.Id, dest, message})
	// Update local state before sending the tokens
	server.Tokens -= numTokens
	link, ok := server.outboundLinks[dest]
	if !ok {
		log.Fatalf("Unknown dest ID %v from server %v\n", dest, server.Id)
	}
	link.events.Push(SendMessageEvent{
		server.Id,
		dest,
		message,
		server.sim.GetReceiveTime()})
}

// Callback for when a message is received on this server.
// When the snapshot algorithm completes on this server, this function
// should notify the simulator by calling `sim.NotifySnapshotComplete`.
func (server *Server) HandlePacket(src string, message interface{}) {
	// TODO: IMPLEMENT ME
	switch v := message.(type) {
	case MarkerMessage:
		if !server.receivedSnapshot[v.snapshotId] {
			server.StartSnapshot(v.snapshotId)
		}
		if !server.inReceivedMarker[v.snapshotId][src] {
			server.inReceivedMarker[v.snapshotId][src] = true
		}
		if len(server.inReceivedMarker[v.snapshotId]) == len(server.inboundLinks) {
			server.sim.chanMap[v.snapshotId] <- server.snapshot[v.snapshotId]
			server.sim.NotifySnapshotComplete(server.Id, v.snapshotId)
		}
	case TokenMessage:
		for snapshotId, received := range server.receivedSnapshot {
			if received && !server.inReceivedMarker[snapshotId][src] {
				server.snapshot[snapshotId].messages =
					append(server.snapshot[snapshotId].messages, &SnapshotMessage{
						src:     src,
						dest:    server.Id,
						message: message,
					})
			}
		}
		server.Tokens += v.numTokens
	}
}

// Start the chandy-lamport snapshot algorithm on this server.
// This should be called only once per server.
func (server *Server) StartSnapshot(snapshotId int) {
	// TODO: IMPLEMENT ME
	server.inReceivedMarker[snapshotId] = make(map[string]bool)
	server.receivedSnapshot[snapshotId] = true
	server.snapshot[snapshotId] = &SnapshotState{
		id:       snapshotId,
		tokens:   map[string]int{server.Id: server.Tokens},
		messages: make([]*SnapshotMessage, 0),
	}
	server.SendToNeighbors(MarkerMessage{snapshotId: snapshotId})
}
