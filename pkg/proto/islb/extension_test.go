package islb

import (
	"fmt"
	"testing"
)

func TestMarshalTracks(t *testing.T) {
	key, value, err := NodeInfo{Name: "sfu001", Id: "uuid-xxxxx-xxxx", Type: "origin"}.MarshalNodeField()
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("NodeField: key = %s => %s\n", key, value)
}

func TestUnMarshal(t *testing.T) {
	node, err := UnmarshalNodeField("node/uuid-xxxxx-xxxx", `{"name": "sfu001", "id": "uuid-xxxxx-xxxx", "type": "origin"}`)
	if err == nil {
		fmt.Printf("node => %v\n", node)
	}
}
