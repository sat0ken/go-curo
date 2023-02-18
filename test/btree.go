package main

import (
	"fmt"
	"strconv"
)

type Tree struct {
	root *Node
}

type Node struct {
	value int
	Left  *Node
	Right *Node
}

func (t *Tree) Insert(value int) {
	if t.root == nil {
		t.root = &Node{value: value}
	} else {
		t.root.Insert(value)
	}
}

func (n *Node) Insert(value int) {
	if value <= n.value {
		if n.Left == nil {
			n.Left = &Node{value: value}
		} else {
			n.Left.Insert(value)
		}
	} else {
		if n.Right == nil {
			n.Right = &Node{value: value}
		} else {
			n.Right.Insert(value)
		}
	}
}

func Walk(t *Node, c chan int) {
	defer close(c)
	walk(t, c)
}

func walk(t *Node, c chan int) {
	if t == nil {
		return
	}
	walk(t.Left, c)
	c <- t.value
	walk(t.Right, c)
}

func Same(t1, t2 *Node) bool {
	c1, c2 := make(chan int), make(chan int)

	go Walk(t1, c1)
	go Walk(t2, c2)

	for {
		v1, ok1 := <-c1
		v2, ok2 := <-c2
		if ok1 != ok2 || v1 != v2 {
			return false
		}
		if !ok1 && !ok2 {
			break
		}
	}
	return true
}

func printPreOrder(n *Node) {
	if n == nil {
		return
	} else {
		fmt.Printf("%d ", n.value)
		printPreOrder(n.Left)
		printPreOrder(n.Right)
	}
}

func (t *Tree) addRoute(ipaddr uint32) {
	for _, v := range fmt.Sprintf("%032b", ipaddr) {
		i, _ := strconv.Atoi(string(v))
		t.Insert(i)
	}
}

func main() {
	var t Tree
	t.addRoute(0xc0a80001)
	// t.addRoute(0xac100010)

	//printPreOrder(t.root)
	fmt.Println(t.root.value)
	fmt.Println(t.root.Left.value)
	fmt.Println(t.root.Left.Left.value)
	fmt.Println(t.root.Left.Left.Left.value)
	fmt.Println(t.root.Left.Left.Left.Left.value)

}
