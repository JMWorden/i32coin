package int32coin

// MsgType is enumerator for LocalMsg
type MsgType int

const (
	// AddBlock is new block to be added to blockchain
	AddBlock MsgType = iota
	// CandidateBlock is block to be mined
	CandidateBlock
	// StopMine is a signal to stop mining (because a block has been added)
	StopMine
	// ShareBlock is validated block to be shared with the network
	ShareBlock
	// Transaction is a transaction to be enqueued
	Transaction
	// GenCandidate forces blockchain to broadcast a candidate
	GenCandidate
	// ReqHeight request current blockchain height
	ReqHeight
	// Height is respose to ReqHeight
	Height
)

// LocalMsg is administrative message sent between local go routines
type LocalMsg struct {
	Mtype       MsgType
	Block       interface{}
	Miner       interface{}
	Transaction interface{}
	Height      uint64
}
