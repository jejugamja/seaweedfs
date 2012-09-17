package topology

import (
	"fmt"
	"pkg/storage"
)

type NodeId string
type Node interface {
	Id() NodeId
	String() string
	FreeSpace() int
	ReserveOneVolume(r int, vid storage.VolumeId) (bool, *DataNode)
	UpAdjustMaxVolumeCountDelta(maxVolumeCountDelta int)
	UpAdjustActiveVolumeCountDelta(activeVolumeCountDelta int)
	UpAdjustMaxVolumeId(vid storage.VolumeId)
	GetActiveVolumeCount() int
	GetMaxVolumeCount() int
	GetMaxVolumeId() storage.VolumeId
	setParent(Node)
	LinkChildNode(node Node)
	UnlinkChildNode(nodeId NodeId)
	CollectWritableVolumes(freshThreshHold int64, volumeSizeLimit uint64) []storage.VolumeId

	IsDataNode() bool
	Children() map[NodeId]Node
	Parent() Node
}
type NodeImpl struct {
	id                NodeId
	activeVolumeCount int
	maxVolumeCount    int
	parent            Node
	children          map[NodeId]Node
	maxVolumeId       storage.VolumeId

	//for rack, data center, topology
	nodeType string
}

func (n *NodeImpl) IsDataNode() bool {
	return n.nodeType == "DataNode"
}
func (n *NodeImpl) IsRack() bool {
	return n.nodeType == "Rack"
}
func (n *NodeImpl) IsDataCenter() bool {
	return n.nodeType == "DataCenter"
}
func (n *NodeImpl) String() string {
	if n.parent != nil {
		return n.parent.String() + ":" + string(n.id)
	}
	return string(n.id)
}
func (n *NodeImpl) Id() NodeId {
	return n.id
}
func (n *NodeImpl) FreeSpace() int {
	return n.maxVolumeCount - n.activeVolumeCount
}
func (n *NodeImpl) setParent(node Node) {
	n.parent = node
}
func (n *NodeImpl) Children() map[NodeId]Node {
	return n.children
}
func (n *NodeImpl) Parent() Node {
	return n.parent
}
func (n *NodeImpl) ReserveOneVolume(r int, vid storage.VolumeId) (bool, *DataNode) {
	ret := false
	var assignedNode *DataNode
	for _, node := range n.children {
		freeSpace := node.FreeSpace()
		//fmt.Println("r =", r, ", node =", node, ", freeSpace =", freeSpace)
		if freeSpace <= 0 {
			continue
		}
		if r >= freeSpace {
			r -= freeSpace
		} else {
			if node.IsDataNode() && node.FreeSpace() > 0 {
				//fmt.Println("vid =", vid, " assigned to node =", node, ", freeSpace =", node.FreeSpace())
				return true, node.(*DataNode)
			}
			ret, assignedNode = node.ReserveOneVolume(r, vid)
			if ret {
				break
			}
		}
	}
	return ret, assignedNode
}

func (n *NodeImpl) UpAdjustMaxVolumeCountDelta(maxVolumeCountDelta int) { //can be negative
	n.maxVolumeCount += maxVolumeCountDelta
	if n.parent != nil {
		n.parent.UpAdjustMaxVolumeCountDelta(maxVolumeCountDelta)
	}
}
func (n *NodeImpl) UpAdjustActiveVolumeCountDelta(activeVolumeCountDelta int) { //can be negative
	n.activeVolumeCount += activeVolumeCountDelta
	if n.parent != nil {
		n.parent.UpAdjustActiveVolumeCountDelta(activeVolumeCountDelta)
	}
}
func (n *NodeImpl) UpAdjustMaxVolumeId(vid storage.VolumeId) { //can be negative
	if n.maxVolumeId < vid {
		n.maxVolumeId = vid
		if n.parent != nil {
			n.parent.UpAdjustMaxVolumeId(vid)
		}
	}
}

func (n *NodeImpl) GetMaxVolumeId() storage.VolumeId {
	return n.maxVolumeId
}
func (n *NodeImpl) GetActiveVolumeCount() int {
	return n.activeVolumeCount
}
func (n *NodeImpl) GetMaxVolumeCount() int {
	return n.maxVolumeCount
}

func (n *NodeImpl) LinkChildNode(node Node) {
	if n.children[node.Id()] == nil {
		n.children[node.Id()] = node
		n.UpAdjustMaxVolumeCountDelta(node.GetMaxVolumeCount())
		n.UpAdjustMaxVolumeId(node.GetMaxVolumeId())
		n.UpAdjustActiveVolumeCountDelta(node.GetActiveVolumeCount())
		node.setParent(n)
		fmt.Println(n, "adds", node, "volumeCount =", n.activeVolumeCount)
	}
}

func (n *NodeImpl) UnlinkChildNode(nodeId NodeId) {
	node := n.children[nodeId]
	node.setParent(nil)
	if node != nil {
		delete(n.children, node.Id())
		n.UpAdjustActiveVolumeCountDelta(-node.GetActiveVolumeCount())
		n.UpAdjustMaxVolumeCountDelta(-node.GetMaxVolumeCount())
		fmt.Println(n, "removes", node, "volumeCount =", n.activeVolumeCount)
	}
}

func (n *NodeImpl) CollectWritableVolumes(freshThreshHold int64, volumeSizeLimit uint64) []storage.VolumeId {
	var ret []storage.VolumeId
	if n.IsRack() {
		for _, c := range n.Children() {
			dn := c.(*DataNode) //can not cast n to DataNode
			if dn.lastSeen > freshThreshHold {
			  continue
			}
			for _, v := range dn.volumes {
				if uint64(v.Size) < volumeSizeLimit {
					ret = append(ret, v.Id)
				}
			}
		}
	} else {
		for _, c := range n.Children() {
			ret = append(ret, c.CollectWritableVolumes(freshThreshHold, volumeSizeLimit)...)
		}
	}

	return ret
}