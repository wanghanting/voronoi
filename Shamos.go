package voronoi

import (
	"container/heap"
	"fmt"
	"image"
	"log"
	"math"

	"github.com/quasoft/dcel"
)

type Voronoi struct {
	Bounds       image.Rectangle
	Sites        SiteSlice
	EventQueue   EventQueue
	ParabolaTree *Node
	SweepLine    int // tracks the current position of the sweep line; updated when a new site is added.
	DCEL         *dcel.DCEL
}

func New(sites SiteSlice, bounds image.Rectangle) *Voronoi {
	voronoi := &Voronoi{Bounds: bounds}
	voronoi.Sites = make(SiteSlice, len(sites), len(sites))
	copy(voronoi.Sites, sites)
	voronoi.init()
	return voronoi
}

func NewFromPoints(points []image.Point, bounds image.Rectangle) *Voronoi {
	var sites SiteSlice
	var id int64
	for _, point := range points {
		sites = append(sites, Site{
			X:  point.X,
			Y:  point.Y,
			ID: id,
		})
		id++
	}
	return New(sites, bounds)
}

func (v *Voronoi) init() {

	v.EventQueue = NewEventQueue(v.Sites)

	v.ParabolaTree = nil

	v.DCEL = dcel.NewDCEL()
}

func (v *Voronoi) Reset() {
	v.EventQueue = NewEventQueue(v.Sites)
	v.ParabolaTree = nil
	v.SweepLine = 0
	v.DCEL = dcel.NewDCEL()
}

func (v *Voronoi) HandleNextEvent() {
	if v.EventQueue.Len() <= 0 {
		return
	}

	event := heap.Pop(&v.EventQueue).(*Event)

	if event.Y < v.SweepLine {
		log.Printf("Ignoring event with Y %d as it's above the sweep line (%d)\r\n", event.Y, v.SweepLine)
		return
	}

	v.SweepLine = event.Y
	if event.EventType == EventSite {
		v.handleSiteEvent(event)
	} else {
		v.handleCircleEvent(event)
	}
}

func (v *Voronoi) Generate() {
	v.Reset()

	for v.EventQueue.Len() > 0 {
		v.HandleNextEvent()
	}
}

func (v *Voronoi) findNodeAbove(site *Site) *Node {
	node := v.ParabolaTree

	for !node.IsLeaf() {
		if node.IsLeaf() {
			log.Printf("At leaf %v\r\n", node)
		} else {
			log.Printf("At internal node %v <-> %v\r\n", node.PrevChildArc(), node.NextChildArc())
		}

		x, err := GetXOfInternalNode(node, v.SweepLine)
		if err != nil {
			panic(fmt.Errorf("could not find arc above %v - this should never happen", node))
		}
		if site.X < x {
			log.Printf("site.X (%d) < x (%d), going left\r\n", site.X, x)
			node = node.Left
		} else {
			log.Printf("site.X (%d) >= x (%d), going right\r\n", site.X, x)
			node = node.Right
		}
		if node.IsLeaf() {
			log.Printf("X of intersection: %d\r\n", x)
		}
	}

	return node
}

func (v *Voronoi) handleSiteEvent(event *Event) {
	log.Println()
	log.Printf("Handling site event %d,%d\r\n", event.X, event.Y)
	log.Printf("Sweep line: %d", v.SweepLine)
	log.Printf("Tree: %v", v.ParabolaTree)

	// Create a face for this site and link it to it
	face := v.DCEL.NewFace()
	face.ID = event.Site.ID
	face.Data = event.Site
	event.Site.Face = face

	if v.ParabolaTree == nil {
		log.Print("Adding event as root\r\n")
		v.ParabolaTree = &Node{Site: event.Site}
		return
	}

	arcAbove := v.findNodeAbove(event.Site)
	if arcAbove == nil {
		log.Print("Could not find arc above event site!\r\n")
		return
	}
	log.Printf("Arc above: %v\r\n", arcAbove)

	v.removeCircleEvent(arcAbove)

	y := GetYByX(arcAbove.Site, event.Site.X, v.SweepLine)
	vertex := v.DCEL.NewVertex(event.Site.X, y)
	log.Printf("Y of intersection = %d,%d\r\n", vertex.X, vertex.Y)

	arcAbove.Right = &Node{
		Site:       arcAbove.Site,
		LeftEvents: arcAbove.LeftEvents,
		Parent:     arcAbove,
	}
	oldArcRight := arcAbove.Right
	oldArcRight.RightEdges = make([]*dcel.HalfEdge, len(arcAbove.RightEdges))
	copy(oldArcRight.RightEdges, arcAbove.RightEdges)

	arcAbove.Left = &Node{Parent: arcAbove}

	arcAbove.Left.Right = &Node{
		Site:   event.Site,
		Parent: arcAbove.Left,
	}
	newArc := arcAbove.Left.Right

	arcAbove.Left.Left = &Node{
		Site:        arcAbove.Site,
		RightEvents: arcAbove.RightEvents,
		Parent:      arcAbove.Left,
	}
	oldArcLeft := arcAbove.Left.Left
	oldArcLeft.LeftEdges = make([]*dcel.HalfEdge, len(arcAbove.LeftEdges))
	copy(oldArcLeft.LeftEdges, arcAbove.LeftEdges)

	arcAbove.Site = nil
	arcAbove.LeftEvents = nil
	arcAbove.MiddleEvents = nil
	arcAbove.RightEvents = nil

	edge1, edge2 := v.DCEL.NewEdge(oldArcLeft.Site.Face, newArc.Site.Face, vertex)
	oldArcLeft.RightEdges = append(oldArcLeft.RightEdges, edge1)
	newArc.LeftEdges = append(newArc.LeftEdges, edge2)

	edge3, edge4 := v.DCEL.NewEdge(newArc.Site.Face, oldArcRight.Site.Face, vertex)
	newArc.RightEdges = append(newArc.RightEdges, edge3)
	oldArcRight.LeftEdges = append(oldArcRight.LeftEdges, edge4)

	prevArc := newArc.PrevArc()
	log.Printf("Prev arc for %v is %v\r\n", newArc, prevArc)
	prevPrevArc := prevArc.PrevArc()
	log.Printf("Prev->prev arc for %v is %v\r\n", newArc, prevPrevArc)
	v.addCircleEvent(prevPrevArc, prevArc, newArc)

	nextArc := newArc.NextArc()
	nextNextArc := nextArc.NextArc()
	v.addCircleEvent(newArc, nextArc, nextNextArc)
}

func (v *Voronoi) calcCircle(site1, site2, site3 *Site) (x int, y int, r int, err error) {
	x = 0
	y = 0
	r = 0
	err = nil

	x1 := float64(site1.X)
	y1 := float64(site1.Y)

	x2 := float64(site2.X)
	y2 := float64(site2.Y)

	x3 := float64(site3.X)
	y3 := float64(site3.Y)

	determinant := (x2*y3 + x1*y2 + y1*x3) - (y1*x2 + y2*x3 + x1*y3)
	if determinant < 0 {
		log.Printf("Sites are in reversed order, so circle would be clockwise")
		err = fmt.Errorf("circle is clockwise - sites %f,%f %f,%f %f,%f are in reversed order", x1, y1, x2, y2, x3, y3)
		return
	}

	if x2-x1 == 0 || x3-x2 == 0 {
		log.Printf("Ignoring circle, division by zero")
		err = fmt.Errorf("no circle found connecting points %f,%f %f,%f and %f,%f", x1, y1, x2, y2, x3, y3)
		return
	}

	mr := (y2 - y1) / (x2 - x1)
	mt := (y3 - y2) / (x3 - x2)

	if mr == mt || mr-mt == 0 || mr == 0 {
		log.Printf("Ignoring circle, division by zero")
		err = fmt.Errorf("no circle found connecting points %f,%f %f,%f and %f,%f", x1, y1, x2, y2, x3, y3)
		return
	}

	cx := (mr*mt*(y3-y1) + mr*(x2+x3) - mt*(x1+x2)) / (2 * (mr - mt))
	cy := (y1+y2)/2 - (cx-(x1+x2)/2)/mr
	cr := math.Pow((math.Pow((x2-cx), 2) + math.Pow((y2-cy), 2)), 0.5)

	x = int(cx + 0.5)
	y = int(cy + 0.5)
	r = int(cr + 0.5)

	return
}

func (v *Voronoi) addCircleEvent(arc1, arc2, arc3 *Node) {
	if arc1 == nil || arc2 == nil || arc3 == nil {
		return
	}

	log.Printf("Checking for circle at %v %v %v\r\n", arc1, arc2, arc3)
	x, y, r, err := v.calcCircle(arc1.Site, arc2.Site, arc3.Site)
	if err != nil {
		return
	}

	bottomY := y + r
	if bottomY < v.SweepLine {
		log.Printf("bottomY (%d) would be below sweep line (%d)", bottomY, v.SweepLine)
		return
	}

	event := &Event{
		EventType: EventCircle,
		X:         x,
		Y:         bottomY,
		Radius:    r,
	}
	v.EventQueue.Push(event)

	arc1.AddLeftEvent(event)
	arc2.AddMiddleEvent(event)
	arc3.AddRightEvent(event)
	event.Node = arc2

	log.Printf("Added circle with center %d,%d, r=%d and bottom Y=%d\r\n", x, y, r, bottomY)
}

func (v *Voronoi) handleCircleEvent(event *Event) {
	log.Println()
	log.Printf("Handling circle event %d,%d with radius %d\r\n", event.X, event.Y, event.Radius)
	log.Printf("Sweep line: %d", v.SweepLine)
	log.Printf("Tree: %v", v.ParabolaTree)

	log.Printf("Node to be removed: %v", event.Node)

	vertex := v.DCEL.NewVertex(event.X, event.Y-event.Radius)
	log.Printf("Vertex at %d,%d (center of circle)\r\n", vertex.X, vertex.Y)

	v.CloseTwins(event.Node.LeftEdges, vertex)
	v.CloseTwins(event.Node.RightEdges, vertex)

	prevArc := event.Node.PrevArc()
	nextArc := event.Node.NextArc()
	log.Printf("Removing arc %v between %v and %v", event.Node, prevArc, nextArc)
	log.Printf("Previous arc: %v", prevArc)
	log.Printf("Next arc: %v", nextArc)
	v.removeArc(event.Node)

	v.removeAllCircleEvents(event.Node)

	prevPrevArc := prevArc.PrevArc()
	v.addCircleEvent(prevPrevArc, prevArc, nextArc)

	nextNextArc := nextArc.NextArc()
	v.addCircleEvent(prevArc, nextArc, nextNextArc)

	v.CloseTwins(prevArc.RightEdges, vertex)
	v.CloseTwins(nextArc.LeftEdges, vertex)

	edge1, edge2 := v.DCEL.NewEdge(prevArc.Site.Face, nextArc.Site.Face, vertex)
	prevArc.RightEdges = append(prevArc.RightEdges, edge1)
	nextArc.LeftEdges = append(nextArc.LeftEdges, edge2)

	return
}

func (v *Voronoi) removeArc(node *Node) {
	parent := node.Parent
	other := (*Node)(nil)
	if parent.Left == node {
		other = parent.Right
	} else {
		other = parent.Left
	}
	grandParent := parent.Parent
	if grandParent == nil {
		v.ParabolaTree = other
		v.ParabolaTree.Parent = nil
		return
	}

	if grandParent.Left == parent {
		grandParent.Left = other
		grandParent.Left.Parent = grandParent
	} else if grandParent.Right == parent {
		grandParent.Right = other
		grandParent.Right.Parent = grandParent
	}
}

// removeCircleEvent removes only the circle event where the specified node represents the middle arc.
func (v *Voronoi) removeCircleEvent(middleNode *Node) {
	if middleNode == nil {
		return
	}

	if len(middleNode.MiddleEvents) > 0 {
		log.Printf("Removing circle event where arc %v is the middle.\r\n", middleNode.Site)

		for _, e := range middleNode.MiddleEvents {
			if e.index <= -1 {
				// The event was already removed
				continue
			}

			v.EventQueue.Remove(e)
		}
		middleNode.MiddleEvents = nil

		prevArc := middleNode.PrevArc()
		if prevArc != nil {
			prevArc.RightEvents = nil
		}
		nextArc := middleNode.NextArc()
		if nextArc != nil {
			nextArc.LeftEvents = nil
		}
	}
}

// removeAllCircleEvents removes all circle events in which the node participates.
func (v *Voronoi) removeAllCircleEvents(node *Node) {
	if node == nil {
		return
	}

	node.MiddleEvents = append(node.MiddleEvents, node.LeftEvents...)
	node.MiddleEvents = append(node.MiddleEvents, node.RightEvents...)

	neighbours := []*Node{
		node.PrevArc().PrevArc(),
		node.PrevArc(),
		node.NextArc(),
		node.NextArc().NextArc(),
	}

	if len(node.MiddleEvents) > 0 {
		log.Printf("Removing circle events for arc %v.\r\n", node.Site)

		for _, e := range node.MiddleEvents {
			if e.index <= -1 {
				continue
			}

			v.EventQueue.Remove(e)

			for _, n := range neighbours {
				n.RemoveEvent(e)
			}
		}
		node.LeftEvents = nil
		node.MiddleEvents = nil
		node.RightEvents = nil
	}
}
