package node

type ConnectionManager struct {
	Connections map[string]int
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		Connections: make(map[string]int),
	}
}
