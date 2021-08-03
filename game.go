package main

import (
	// "os"

	"fmt"

	k "github.com/rooklift/kikashi"
)

type Game struct {
	size      int
	turnColor Color
	turn      int
	head      *k.Node
	tail      *k.Node
}

func NewGame(size int) Game {
	turn := 0
	head := k.NewTree(size)
	tail := head
	turnColor := BLACK
	return Game{size, turnColor, turn, head, tail}
}

func (g Game) Packet() map[string]string {
	return map[string]string{
		"board":     toAscii(g.tail),
		"size":      fmt.Sprintf("%v", g.size),
		"turn":      fmt.Sprintf("%v", g.turn),
		"turnColor": g.turnColor.String(),
	}
}

func (g *Game) TryMove(x int, y int) error {
	newNode, err := g.tail.TryMove(g.turnColor.ToKikashi(), x, y)

	if err != nil {
		return err
	}

	g.tail = newNode

	g.turn += 1
	g.turnColor = g.turnColor.Opposite()

	return nil
}

func (g Game) printTree() {
	curr := g.head

	for curr != nil {
		fmt.Println(toAscii(curr))
		if len(curr.Children) > 0 {
			curr = curr.Children[0]
		} else {
			curr = nil
		}
	}
}

func (g Game) printCurrentPosition() {
	fmt.Println(toAscii(g.tail))
}

func toAscii(n *k.Node) string {
	var chars = map[k.Colour]string{
		k.WHITE: "w",
		k.BLACK: "b",
		k.EMPTY: "-",
	}

	board := n.Board
	ascii := ""

	for _, row := range board {
		for _, cell := range row {
			ascii += chars[cell]
		}
		ascii += "\n"
	}
	return ascii
}

func (g Game) IndexToCoords(index int) (x, y int) {
	x = index / g.size
	y = index % g.size
	return
}
