package biz

import (
	"github.com/pion/ion/pkg/discovery"
	"github.com/pion/ion/pkg/log"
	islb "github.com/pion/ion/pkg/proto/islb"
	sfu "github.com/pion/ion/pkg/proto/sfu"
	"google.golang.org/grpc"
)

var (
	//nolint:unused
	dc = "default"
	//nolint:unused
	nid      = "biz-unkown-node-id"
	islbs    map[string]islb.ISLBClient
	sfus     map[string]sfu.SFUClient
	services map[string]discovery.Node
)

// Init func
func Init(dcID, nodeID, rpcID, eventID string, natsURL string) {
	dc = dcID
	nid = nodeID
	services = make(map[string]discovery.Node)
	islbs = make(map[string]islb.ISLBClient)
	sfus = make(map[string]sfu.SFUClient)
}

// WatchServiceNodes .
func WatchServiceNodes(service string, state discovery.NodeStateType, node discovery.Node) {
	id := node.ID

	if state == discovery.UP {

		if _, found := services[id]; !found {
			services[id] = node
		}

		service := node.Info["service"]
		name := node.Info["name"]
		log.Debugf("Service [%s] %s => %s", service, name, id)

		_, found := islbs[id]
		if !found {
			addr := discovery.GetGRPCAddress(node)
			conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock())
			if err != nil {
				log.Panicf("did not connect: %v", err)
			}
			islbs[id] = islb.NewISLBClient(conn)

			log.Infof("Created islb client: grpc => [%s]", addr)

			// log.Infof("handleIslbBroadCast: eventID => [%s]", eventID)
			// protoo.OnBroadcast(eventID, handleIslbBroadCast)
		}

	} else if state == discovery.DOWN {
		delete(islbs, id)
		delete(services, id)
	}
}

// Close func
func Close() {
	// if protoo != nil {
	// 	protoo.Close()
	// }
}
