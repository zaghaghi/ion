package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	nprotoo "github.com/cloudwebrtc/nats-protoo"
	"github.com/pion/ion/pkg/discovery"
	"github.com/pion/ion/pkg/log"
	"github.com/pion/ion/pkg/proto"
	"github.com/pion/ion/pkg/signal"
	"github.com/pion/ion/pkg/util"
	"google.golang.org/grpc"

	islbpb "github.com/pion/ion/pkg/proto/islb"
	pb "github.com/pion/ion/pkg/proto/sfu"
)

// ParseProtoo Unmarshals a protoo payload.
func ParseProtoo(msg json.RawMessage, msgType interface{}) *nprotoo.Error {
	if err := json.Unmarshal(msg, &msgType); err != nil {
		log.Errorf("Biz.Entry parse error %v", err.Error())
		return util.NewNpError(http.StatusBadRequest, fmt.Sprintf("Error parsing request object %v", err.Error()))
	}
	return nil
}

// Entry is the biz entry
func Entry(method string, peer *signal.Peer, msg json.RawMessage, accept signal.RespondFunc, reject signal.RejectFunc) {
	var result interface{}
	topErr := util.NewNpError(http.StatusBadRequest, fmt.Sprintf("Unkown method [%s]", method))

	//TODO DRY this up
	switch method {
	case proto.ClientJoin:
		var msgData proto.JoinMsg
		if topErr = ParseProtoo(msg, &msgData); topErr == nil {
			result, topErr = join(peer, msgData)
		}
	case proto.ClientLeave:
		var msgData proto.LeaveMsg
		if topErr = ParseProtoo(msg, &msgData); topErr == nil {
			result, topErr = leave(peer, msgData)
		}
	case proto.ClientPublish:
		var msgData pb.PublishRequest
		if topErr = ParseProtoo(msg, &msgData); topErr == nil {
			result, topErr = publish(peer, msgData)
		}
	case proto.ClientUnPublish:
		var msgData pb.UnpublishRequest
		if topErr = ParseProtoo(msg, &msgData); topErr == nil {
			result, topErr = unpublish(peer, msgData)
		}
	case proto.ClientSubscribe:
		var msgData pb.SubscribeRequest
		if topErr = ParseProtoo(msg, &msgData); topErr == nil {
			result, topErr = subscribe(peer, msgData)
		}
	case proto.ClientUnSubscribe:
		var msgData pb.UnsubscribeRequest
		if topErr = ParseProtoo(msg, &msgData); topErr == nil {
			result, topErr = unsubscribe(peer, msgData)
		}
	case proto.ClientBroadcast:
		var msgData proto.BroadcastMsg
		if topErr = ParseProtoo(msg, &msgData); topErr == nil {
			result, topErr = broadcast(peer, msgData)
		}
	case proto.ClientTrickleICE:
		var msgData proto.TrickleMsg
		if topErr = ParseProtoo(msg, &msgData); topErr == nil {
			result, topErr = trickle(peer, msgData)
		}
	}

	if topErr != nil {
		reject(topErr.Code, topErr.Reason)
	} else {
		accept(result)
	}
}

func getRPCForIslb() (islbpb.ISLBClient, bool) {
	for _, item := range services {
		if item.Info["service"] == "islb" {
			id := item.Info["id"]
			islb, found := islbs[id]
			if !found {
				addr := discovery.GetGRPCAddress(item)
				conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock())
				if err != nil {
					log.Panicf("did not connect: %v", err)
				}
				islbs[id] = islbpb.NewISLBClient(conn)
				log.Infof("Created grpc client for islb at [%s]", addr)
				islbs[id] = islb
			}
			return islb, true
		}
	}
	log.Warnf("islb node not found.")
	return nil, false
}

func handleSFUBroadCast(msg nprotoo.Notification, subj string) {
	go func(msg nprotoo.Notification) {
		var data proto.MediaInfo
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			log.Errorf("handleSFUBroadCast Unmarshall error %v", err)
			return
		}

		log.Infof("handleSFUBroadCast: method=%s, data=%v", msg.Method, data)

		switch msg.Method {
		case proto.SFUTrickleICE:
			signal.NotifyAllWithoutID(data.RID, data.UID, proto.ClientOnStreamAdd, data)
		case proto.SFUStreamRemove:
			islb, found := getRPCForIslb()
			if found {
				islb.StreamRemove()
				// islb.AsyncRequest(proto.IslbOnStreamRemove, data)
			}
		}
	}(msg)
}

func getRPCForSFU(mid string) (string, pb.SFUClient, error) {
	islb, found := getRPCForIslb()
	if !found {
		return "", nil, fmt.Errorf("islb node not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := islb.FindService(ctx, &islbpb.FindServiceRequest{Service: "sfu", Mid: mid})
	if err != nil {
		return "", nil, err
	}

	log.Infof("SFU result => %s", result)
	addr := result.Grpc
	sfu, found := sfus[addr]
	if !found {
		conn, err := grpc.Dial(result.Grpc, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Panicf("did not connect: %v", err)
			return "", nil, fmt.Errorf("sfu node not found")
		}

		sfu = pb.NewSFUClient(conn)
		sfus[addr] = sfu
	}

	return result.Id, sfu, nil
}
