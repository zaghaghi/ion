package biz

import (
	"context"
	"fmt"
	"time"

	nprotoo "github.com/cloudwebrtc/nats-protoo"
	"github.com/pion/ion/pkg/log"
	"github.com/pion/ion/pkg/proto"
	"github.com/pion/ion/pkg/signal"
	"github.com/pion/ion/pkg/util"

	islbpb "github.com/pion/ion/pkg/proto/islb"
	media "github.com/pion/ion/pkg/proto/media"
	pb "github.com/pion/ion/pkg/proto/sfu"
)

var (
	ridError  = util.NewNpError(codeRoomErr, codeStr(codeRoomErr))
	jsepError = util.NewNpError(codeJsepErr, codeStr(codeJsepErr))
	// sdpError  = util.NewNpError(codeSDPErr, codeStr(codeSDPErr))
	midError = util.NewNpError(codeMIDErr, codeStr(codeMIDErr))
)

// join room
func join(peer *signal.Peer, msg proto.JoinMsg) (interface{}, *nprotoo.Error) {
	log.Infof("biz.join peer.ID()=%s msg=%v", peer.ID(), msg)
	rid := msg.RID
	uid := peer.ID()

	// Validate
	if msg.RID == "" {
		return nil, ridError
	}

	// User has already joined this room
	if signal.HasPeer(rid, peer) {
		return emptyMap, nil
	}

	signal.AddPeer(rid, peer)

	islb, found := getRPCForIslb()
	if !found {
		return nil, util.NewNpError(500, "Not found any node for islb.")
	}

	// Send join => islb
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := islb.Join(ctx, &islbpb.JoinRequest{Rid: rid, Uid: uid, Metadata: &islbpb.UserMetadata{
		Name: msg.Info.Name,
	}})
	if err != nil {
		log.Errorf("IslbClientOnJoin failed %v", err.Error())
	}

	// Send getPubs => islb
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		res, err := islb.GetPubs(ctx, &islbpb.GetPubsRequest{
			Rid: rid,
			Uid: uid,
		})

		if err != nil {
			log.Errorf("error getting pubs for rid => %s, uid => %s", rid, uid)
			return
		}

		log.Infof("GetPubs: result=%s", res)
		for _, pub := range res.Pubs {
			if pub.Minfo.Mid == "" {
				continue
			}
			peer.Notify(proto.ClientOnStreamAdd, pub)
		}
	}()

	return emptyMap, nil
}

func leave(peer *signal.Peer, msg proto.LeaveMsg) (interface{}, *nprotoo.Error) {
	log.Infof("biz.leave peer.ID()=%s msg=%v", peer.ID(), msg)
	defer util.Recover("biz.leave")

	rid := msg.RID

	// Validate
	if msg.RID == "" {
		return nil, ridError
	}

	uid := peer.ID()

	islb, found := getRPCForIslb()
	if !found {
		return nil, util.NewNpError(500, "Not found any node for islb.")
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		islb.StreamRemove(ctx, &islbpb.StreamRemoveRequest{
			Minfo: &media.Info{
				Rid: rid,
				Uid: uid,
			},
		})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := islb.Leave(ctx, &islbpb.LeaveRequest{
		Rid: rid,
		Uid: uid,
	})
	if err != nil {
		log.Errorf("IslbOnStreamRemove failed %v", err.Error())
	}
	signal.DelPeer(rid, peer.ID())
	return emptyMap, nil
}

func publish(peer *signal.Peer, msg pb.PublishRequest) (interface{}, *nprotoo.Error) {
	log.Infof("biz.publish peer.ID()=%s", peer.ID())

	nid, sfu, err := getRPCForSFU("")
	if err != nil {
		log.Warnf("Not found any sfu node, reject: %d => %s", 500, err)
		return nil, util.NewNpError(500, fmt.Sprintf("%s", err))
	}

	room := signal.GetRoomByPeer(peer.ID())
	if room == nil {
		return nil, util.NewNpError(codeRoomErr, codeStr(codeRoomErr))
	}

	rid := room.ID()
	uid := peer.ID()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := sfu.Publish(ctx, &msg)

	if err != nil {
		log.Warnf("reject: %s", err)
		return nil, util.NewNpError(500, fmt.Sprintf("%s", err))
	}

	log.Infof("publish: res => %v", res)

	islb, found := getRPCForIslb()
	if !found {
		return nil, util.NewNpError(500, "islb node not found")
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		islb.StreamAdd(ctx, &islbpb.StreamAddRequest{
			Minfo: &media.Info{
				Rid: rid,
				Nid: nid,
				Uid: uid,
				Mid: res.Mediainfo.Mid,
			},
			Stream: res.Stream,
		})
	}()

	return res, nil
}

// unpublish from app
func unpublish(peer *signal.Peer, msg pb.UnpublishRequest) (interface{}, *nprotoo.Error) {
	log.Infof("signal.unpublish peer.ID()=%s msg=%v", peer.ID(), msg)

	mid := msg.Mid
	uid := peer.ID()

	_, sfu, err := getRPCForSFU(mid)
	if err != nil {
		log.Warnf("Not found any sfu node, reject: %s", err)
		return nil, util.NewNpError(500, fmt.Sprintf("%s", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = sfu.Unpublish(ctx, &pb.UnpublishRequest{Mid: string(mid)})
	if err != nil {
		log.Warnf("reject: %s", err)
		return nil, util.NewNpError(500, fmt.Sprintf("%s", err))
	}

	islb, found := getRPCForIslb()
	if !found {
		return nil, util.NewNpError(500, "Not found any node for islb.")
	}

	// if this mid is a webrtc pub tell islb stream-remove,
	// `rtc.DelPub(mid)` will be done when islb broadcasts stream-remove
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		islb.StreamRemove(ctx, &islbpb.StreamRemoveRequest{Minfo: &media.Info{Uid: uid, Mid: mid}})
	}()

	return emptyMap, nil
}

func subscribe(peer *signal.Peer, msg pb.SubscribeRequest) (interface{}, *nprotoo.Error) {
	log.Infof("biz.subscribe peer.ID()=%s ", peer.ID())
	mid := msg.Mid

	// Validate
	if mid == "" {
		return nil, midError
	} else if msg.Description.Sdp == "" {
		return nil, jsepError
	}

	nodeID, sfu, err := getRPCForSFU(mid)
	if err != nil {
		log.Warnf("Not found any sfu node, reject: %d => %s", err, err)
		return nil, util.NewNpError(500, fmt.Sprintf("%s", err))
	}

	// TODO:
	if nodeID != "node for mid" {
		log.Warnf("Not the same node, need to enable sfu-sfu relay!")
	}

	room := signal.GetRoomByPeer(peer.ID())
	rid := room.ID()

	islb, found := getRPCForIslb()
	if !found {
		return nil, util.NewNpError(500, "Not found any node for islb.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sres, err := islb.GetStream(ctx, &islbpb.GetStreamRequest{Minfo: &media.Info{Rid: rid, Mid: mid}})
	if err != nil {
		log.Warnf("reject: %s", err)
		return nil, util.NewNpError(404, fmt.Sprintf("%s", err))
	}

	msg.Stream = sres.Stream

	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := sfu.Subscribe(ctx, &msg)

	if err != nil {
		log.Warnf("reject: %s", err)
		return nil, util.NewNpError(500, fmt.Sprintf("%s", err))
	}

	log.Infof("subscribe: result => %v", res)
	return res, nil
}

func unsubscribe(peer *signal.Peer, msg pb.UnsubscribeRequest) (interface{}, *nprotoo.Error) {
	log.Infof("biz.unsubscribe peer.ID()=%s msg=%v", peer.ID(), msg)
	mid := msg.Mid

	// Validate
	if mid == "" {
		return nil, midError
	}

	_, sfu, err := getRPCForSFU(mid)
	if err != nil {
		log.Warnf("Not found any sfu node, reject: %s", err)
		return nil, util.NewNpError(500, fmt.Sprintf("%s", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := sfu.Unsubscribe(ctx, &msg)
	if err != nil {
		log.Warnf("reject: %s", err)
		return nil, util.NewNpError(500, fmt.Sprintf("%s", err))
	}

	log.Infof("publish: result => %v", result)
	return result, nil
}

func broadcast(peer *signal.Peer, msg proto.BroadcastMsg) (interface{}, *nprotoo.Error) {
	log.Infof("biz.broadcast peer.ID()=%s msg=%v", peer.ID(), msg)

	// Validate
	if msg.RID == "" || msg.UID == "" {
		return nil, ridError
	}

	islb, found := getRPCForIslb()
	if !found {
		return nil, util.NewNpError(500, "islb not found")
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		islb.Broadcast(ctx, &islbpb.BroadcastRequest{Rid: msg.RID, Uid: msg.UID, Payload: msg.Info})
	}()

	return emptyMap, nil
}

func trickle(peer *signal.Peer, msg proto.TrickleMsg) (interface{}, *nprotoo.Error) {
	log.Infof("biz.trickle peer.ID()=%s msg=%v", peer.ID(), msg)
	// mid := string(msg.MID)

	// // Validate
	// if msg.RID == "" || msg.UID == "" {
	// 	return nil, ridError
	// }

	// _, sfu, err := getRPCForSFU(mid)
	// if err != nil {
	// 	log.Warnf("Not found any sfu node, reject: %d => %s", err.Code, err.Reason)
	// 	return nil, util.NewNpError(err.Code, err.Reason)
	// }

	// trickle := msg.Trickle

	// sfu.AsyncRequest(proto.ClientTrickleICE, util.Map("mid", mid, "trickle", trickle))
	return emptyMap, nil
}
