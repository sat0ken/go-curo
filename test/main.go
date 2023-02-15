package main

import (
	"fmt"

	"golang.org/x/tour/tree"
)

// Walk walks the tree t sending all values
// from the tree to the channel ch.
func Walk(t *tree.Tree, c chan int) {
	defer close(c)
	walk(t, c)
}

func walk(t *tree.Tree, c chan int) {
	if t == nil {
		return
	}
	walk(t.Left, c)
	c <- t.Value
	walk(t.Right, c)
}

// Same determines whether the trees
// t1 and t2 contain the same values.
func Same(t1, t2 *tree.Tree) bool {
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

func main() {
	fmt.Println(Same(tree.New(1), tree.New(1)))
	fmt.Println(Same(tree.New(2), tree.New(2)))
	fmt.Println(Same(tree.New(3), tree.New(3)))

	fmt.Println(Same(tree.New(1), tree.New(2)))
	fmt.Println(Same(tree.New(1), tree.New(3)))
	fmt.Println(Same(tree.New(1), tree.New(4)))

	fmt.Println(Same(tree.New(2), tree.New(1)))
	fmt.Println(Same(tree.New(3), tree.New(1)))
	fmt.Println(Same(tree.New(4), tree.New(1)))
}
