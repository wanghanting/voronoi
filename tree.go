package voronoi

import (
	"fmt"

	"github.com/quasoft/dcel"
)

type CircleEvents []*Event

func (ce *CircleEvents) RemoveEvent(event *Event) {
	for i := len(*ce) - 1; i >= 0; i-- {
		if (*ce)[i] == event {
			(*ce)[i] = (*ce)[len(*ce)-1]
			*ce = (*ce)[:len(*ce)-1]
		}
	}
}

func (ce *CircleEvents) HasEvent(event *Event) bool {
	for i := 0; i < len(*ce); i++ {
		if (*ce)[i] == event {
			return true
		}
	}
	return false
}

type Node struct {
	Site *Site

	LeftEvents, MiddleEvents, RightEvents CircleEvents
	Parent                                *Node
	Left                                  *Node
	Right                                 *Node

	LeftEdges  []*dcel.HalfEdge
	RightEdges []*dcel.HalfEdge
}

func (n *Node) String() string {
	if n == nil {
		return "()"
	}
	s := ""
	if n.Left != nil {
		s += n.Left.String() + " "
	}

	if n.IsLeaf() {
		s += "[" + fmt.Sprint(n.Site) + "]"
	} else if n.Parent == nil {
		s += "<root>"
	} else {
		s += "<int>" // internal
	}

	if n.Right != nil {
		s += " " + n.Right.String()
	}

	return "(" + s + ")"
}

func (n *Node) IsLeaf() bool {
	return n.Left == nil && n.Right == nil
}

func (n *Node) PrevChildArc() *Node {
	left := n.Left
	for !left.IsLeaf() {
		left = left.Right
	}
	return left
}

func (n *Node) NextChildArc() *Node {
	right := n.Right
	for !right.IsLeaf() {
		right = right.Left
	}
	return right
}

func (n *Node) PrevArc() *Node {
	if n == nil {
		return nil
	}

	if !n.IsLeaf() {
		return n.LastArc()
	}

	if n.Parent == nil {
		return nil
	}

	parent := n.Parent
	node := n
	for parent.Left == node {
		if parent.Parent == nil {
			return nil
		}
		node = parent
		parent = parent.Parent
	}

	if parent.Left.IsLeaf() {
		return parent.Left
	}

	return parent.Left.LastArc()
}

func (n *Node) NextArc() *Node {
	if n == nil {
		return nil
	}

	if !n.IsLeaf() {
		return n.FirstArc()
	}

	if n.Parent == nil {
		return nil
	}

	parent := n.Parent
	node := n
	for parent.Right == node {
		if parent.Parent == nil {
			return nil
		}
		node = parent
		parent = parent.Parent
	}

	if parent.Right.IsLeaf() {
		return parent.Right
	}

	return parent.Right.FirstArc()
}

func (n *Node) FirstArc() *Node {
	first := n
	for first != nil && !first.IsLeaf() {
		if first.Left != nil {
			first = first.Left
		} else {
			first = first.Right
		}
	}
	return first
}

func (n *Node) LastArc() *Node {
	last := n
	for last != nil && !last.IsLeaf() {
		if last.Right != nil {
			last = last.Right
		} else {
			last = last.Left
		}
	}
	return last
}

func (n *Node) AddLeftEvent(event *Event) {
	n.LeftEvents = append(n.LeftEvents, event)
}

func (n *Node) AddMiddleEvent(event *Event) {
	n.MiddleEvents = append(n.MiddleEvents, event)
}

func (n *Node) AddRightEvent(event *Event) {
	n.RightEvents = append(n.RightEvents, event)
}

func (n *Node) RemoveEvent(event *Event) {
	n.LeftEvents.RemoveEvent(event)
	n.MiddleEvents.RemoveEvent(event)
	n.RightEvents.RemoveEvent(event)
}

func (n *Node) HasEvent(event *Event) bool {
	return n.LeftEvents.HasEvent(event) || n.MiddleEvents.HasEvent(event) ||
		n.RightEvents.HasEvent(event)
}
