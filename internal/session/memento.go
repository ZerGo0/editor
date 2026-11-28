package session

import (
	"encoding/json"
	"log"
	"maps"
)

type Memento struct {
	saved  map[string]json.RawMessage
	savers map[string]func() any
}

func NewMemento(saved map[string]json.RawMessage) *Memento {
	return &Memento{saved: maps.Clone(saved), savers: map[string]func() any{}}
}

func (m *Memento) Load(key string, v any) bool {
	data, ok := m.saved[key]
	if !ok {
		return false
	}
	if err := json.Unmarshal(data, v); err != nil {
		log.Printf("load view state %s: %v", key, err)
		return false
	}
	return true
}

func (m *Memento) Track(key string, save func() any) {
	m.savers[key] = save
}

func (m *Memento) Set(key string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("save view state %s: %v", key, err)
		return
	}
	if m.saved == nil {
		m.saved = map[string]json.RawMessage{}
	}
	m.saved[key] = data
}

func (m *Memento) Snapshot() map[string]json.RawMessage {
	for key, save := range m.savers {
		m.Set(key, save())
	}
	if len(m.saved) == 0 {
		return nil
	}
	return maps.Clone(m.saved)
}
