package proto

import (
	"encoding/json"

	pb "github.com/pion/ion/pkg/proto/sfu"
)

type ClientUserInfo struct {
	Name string `json:"name"`
}

func (m *ClientUserInfo) MarshalBinary() ([]byte, error) {
	return json.Marshal(m)
}

func (m *ClientUserInfo) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, m)
}

type RoomInfo struct {
	RID RID `json:"rid"`
	UID UID `json:"uid"`
}

/// Messages ///

type JoinMsg struct {
	RoomInfo
	Info ClientUserInfo `json:"info"`
}

type LeaveMsg struct {
	RoomInfo
	Info ClientUserInfo `json:"info"`
}

type BroadcastMsg struct {
	RoomInfo
	Info json.RawMessage `json:"info"`
}

type TrickleMsg struct {
	MediaInfo
	Info    json.RawMessage `json:"info"`
	Trickle json.RawMessage `json:"trickle"`
}

type StreamAddMsg struct {
	MediaInfo
	Info   ClientUserInfo `json:"info"`
	Stream pb.Stream      `json:"stream"`
}

type StreamRemoveMsg struct {
	MediaInfo
}
