package proto

import (
	"fmt"
)

const (
	// client to ion
	ClientJoin        = "join"
	ClientLeave       = "leave"
	ClientPublish     = "publish"
	ClientUnPublish   = "unpublish"
	ClientSubscribe   = "subscribe"
	ClientUnSubscribe = "unsubscribe"
	ClientBroadcast   = "broadcast"
	ClientTrickleICE  = "trickle"

	// ion to client
	ClientOnJoin         = "peer-join"
	ClientOnLeave        = "peer-leave"
	ClientOnStreamAdd    = "stream-add"
	ClientOnStreamRemove = "stream-remove"

	// ion to islb
	IslbFindService  = "findService"
	IslbGetPubs      = "getPubs"
	IslbGetMediaInfo = "getMediaInfo"
	IslbRelay        = "relay"
	IslbUnrelay      = "unRelay"

	IslbKeepAlive      = "keepAlive"
	IslbClientOnJoin   = ClientOnJoin
	IslbClientOnLeave  = ClientOnLeave
	IslbOnStreamAdd    = ClientOnStreamAdd
	IslbOnStreamRemove = ClientOnStreamRemove
	IslbOnBroadcast    = ClientBroadcast

	// SFU Endpoints
	SFUTrickleICE   = ClientTrickleICE
	SFUStreamRemove = ClientOnStreamRemove

	// AVPProcess enables an audio/video processor
	AVPProcess = "process"
	// AVPUnprocess disables an audio/video processor
	AVPUnprocess = "unprocess"

	IslbID = "islb"
)

/*
media
dc/${nid}/${rid}/${uid}/media/pub/${mid}

node1 origin
node2 shadow
msid  [{ssrc: 1234, pt: 111, type:audio}]
msid  [{ssrc: 5678, pt: 96, type:video}]
*/
type MediaInfo struct {
	DC  string `json:"dc,omitempty"`  //Data Center ID
	NID string `json:"nid,omitempty"` //Node ID
	RID string `json:"rid,omitempty"` //Room ID
	UID string `json:"uid,omitempty"` //User ID
	MID string `json:"mid,omitempty"` //Media ID
}

/*
user
/dc/room1/user/info/${uid}
info {name: "Guest"}
*/

type UserInfo struct {
	DC  string
	RID string
	UID string
}

func GetPubNodePath(rid, uid string) string {
	return rid + "/node/pub/" + uid
}

func GetPubMediaPath(rid, mid string, ssrc uint32) string {
	if ssrc != 0 {
		return rid + "/media/pub/" + mid + fmt.Sprintf("/%d", ssrc)
	}
	return rid + "/media/pub/" + mid
}

func GetPubMediaPathKey(rid string) string {
	return rid + "/media/pub/"
}
