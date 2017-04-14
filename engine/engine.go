package engine

type Message struct {
}

type Endpoint interface {
	Inject(m *Message) error
}

type Engine struct {
	neighbors map[string]Endpoint
}

func NewEngine() *Engine {
	return &Engine{
		neighbors: make(map[string]Endpoint),
	}
}

func (e *Engine) AddNeighbor(id string, ep Endpoint) error {
	e.neighbors[id] = ep
	return nil
}
