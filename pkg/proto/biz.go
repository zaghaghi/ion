package proto

import (
	"encoding/json"

	media "github.com/pion/ion/pkg/proto/media"
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
	RID string `json:"rid"`
	UID string `json:"uid"`
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
	Stream media.Stream   `json:"stream"`
}

type StreamRemoveMsg struct {
	MediaInfo
}
