package fsm

type Instruction struct {
	Op    string `json:"op"` // "set" 或 "delete"
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}
