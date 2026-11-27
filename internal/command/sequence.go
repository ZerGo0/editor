package command

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/egoist/mygo"
)

const (
	sequencePrefix = "runCommands:"
	sequenceJoin   = ", "
	sequenceTitle  = "Run "
)

func SequenceID(ids []string) string {
	data, err := json.Marshal(ids)
	if err != nil {
		return sequencePrefix + "[]"
	}
	return sequencePrefix + string(data)
}

func ParseSequenceID(id string) ([]string, bool) {
	rest, ok := strings.CutPrefix(id, sequencePrefix)
	if !ok {
		return nil, false
	}
	var ids []string
	if err := json.Unmarshal([]byte(rest), &ids); err != nil {
		return nil, false
	}
	return ids, true
}

func (r *Registry) sequence(id string) *Command {
	ids, ok := ParseSequenceID(id)
	if !ok {
		return nil
	}
	return &Command{ID: id, Title: sequenceTitle + strings.Join(ids, sequenceJoin), Run: func(win *mygo.Window) {
		for _, item := range ids {
			if !r.Run(item, win) {
				r.warnUnknown(item)
			}
		}
	}}
}

func (r *Registry) warnUnknown(id string) {
	if r.warned[id] {
		return
	}
	r.warned[id] = true
	log.Printf("runCommands: skipped the unknown command %s", id)
}
