package db

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pion/ion/pkg/proto"
	islb "github.com/pion/ion/pkg/proto/islb"
	media "github.com/pion/ion/pkg/proto/media"
)

var (
	db     *Redis
	dc     = "dc1"
	node   = "sfu1"
	room   = "room1"
	uid    = "uuid-xxxxx-xxxxx-xxxxx-xxxxx"
	mid    = "mid-xxxxx-xxxxx-xxxxx-xxxxx"
	msid0  = "pion audio"
	msid1  = "pion video"
	track0 = media.Track{Ssrc: 3694449886, Payload: 111, Type: "audio", Id: "aid0"}
	track1 = media.Track{Ssrc: 3694449887, Payload: 96, Type: "video", Id: "vid1"}
	track2 = media.Track{Ssrc: 3694449888, Payload: 117, Type: "video", Id: "vid2"}
	node0  = islb.NodeInfo{Name: "node-name-01", Id: "node-idId-01", Type: "origin"}
	node1  = islb.NodeInfo{Name: "node-name-02", Id: "node-id-02", Type: "shadow"}

	uikey = "info"
	uinfo = `{"name": "Guest"}`

	mkey = media.Info{
		Dc:  dc,
		Nid: node,
		Rid: room,
		Uid: uid,
		Mid: mid,
	}.BuildKey()
	ukey = proto.UserInfo{
		DC:  dc,
		RID: room,
		UID: uid,
	}.BuildKey()
)

func init() {
	cfg := Config{
		Addrs: []string{":6380"},
		Pwd:   "",
		DB:    0,
	}
	db = NewRedis(cfg)
}

func TestRedisStorage(t *testing.T) {
	stream := media.Stream{
		Id:     msid0,
		Tracks: []*media.Track{&track0},
	}
	field, value, err := stream.MarshalTracks()
	if err != nil {
		t.Error(err)
	}
	t.Logf("HSet Track %s, %s => %s\n", mkey, field, value)
	err = db.HSet(mkey, field, value)
	if err != nil {
		t.Error(err)
	}

	stream = media.Stream{
		Id:     msid1,
		Tracks: []*media.Track{&track1, &track2},
	}
	field, value, err = stream.MarshalTracks()
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("HSet Track %s, %s => %s\n", mkey, field, value)
	err = db.HSet(mkey, field, value)
	if err != nil {
		t.Error(err)
	}

	field, value, err = node0.MarshalNodeField()
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("HSet Node %s, %s => %s\n", mkey, field, value)
	err = db.HSet(mkey, field, value)
	if err != nil {
		t.Error(err)
	}

	field, value, err = node1.MarshalNodeField()
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("HSet Node %s, %s => %s\n", mkey, field, value)
	err = db.HSet(mkey, field, value)
	if err != nil {
		t.Error(err)
	}

	fmt.Printf("HSet UserInfo %s, %s => %s\n", ukey, uikey, uinfo)
	err = db.HSet(ukey, uikey, uinfo)
	if err != nil {
		t.Error(err)
	}
}

func TestRedisRead(t *testing.T) {

	fields := db.HGetAll(mkey)

	for key, value := range fields {
		if strings.HasPrefix(key, "node/") {
			node, err := islb.UnmarshalNodeField(key, value)
			if err != nil {
				t.Error(err)
			}
			fmt.Printf("node => %v\n", node)
			// if node.Id == "node-id-01" && *node != node0 {
			// 	t.Error("node0 not equal")
			// }

			// if node.Id == "node-id-02" && *node != node1 {
			// 	t.Error("node1 not equal")
			// }
		}
		if strings.HasPrefix(key, "track/") {
			stream, err := media.UnmarshalTracks(key, value)
			if err != nil {
				t.Error(err)
			}
			fmt.Printf("msid => %s, tracks => %v\n", stream.Id, stream.Tracks)

			if stream.Id == msid0 && len(stream.Tracks) != 1 {
				t.Error("track0 not equal")
			}

			if stream.Id == msid1 && len(stream.Tracks) != 2 {
				t.Error("track1 not equal")
			}
		}
	}

	fields = db.HGetAll(ukey)
	for key, value := range fields {
		fmt.Printf("key => %s, value => %v\n", key, value)
		if key != uikey && value != uinfo {
			t.Errorf("Failed %s => %s", key, value)
		}
	}
}
