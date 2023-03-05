package mutesting

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"os"
	"strings"

	"github.com/merlins-labs/go-mutesting/mutator"
	"github.com/zimmski/go-mutesting"
)

// CountWalk returns the number of corresponding mutations for a given mutator.
// It traverses the AST of the given node and calls the method Check of the given mutator for every node and sums up the returned counts. After completion of the traversal the final counter is returned.
func CountWalk(pkg *types.Package, info *types.Info, node ast.Node, m mutator.Mutator) int {
	w := &countWalk{
		count:   0,
		mutator: m,
		pkg:     pkg,
		info:    info,
	}

	ast.Walk(w, node)

	return w.count
}

type countWalk struct {
	count   int
	mutator mutator.Mutator
	pkg     *types.Package
	info    *types.Info
}

// Visit implements the Visit method of the ast.Visitor interface
func (w *countWalk) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return w
	}

	w.count += len(w.mutator(w.pkg, w.info, node))

	return w
}

// MutateWalk mutates the given node with the given mutator returning a channel to control the mutation steps.
//
// It traverses the AST of the given node and calls the method `Check` of the given mutator to verify that a node can be mutated by the mutator.
// If a node can be mutated the method `Mutate` of the given mutator is executed with the node and the control channel. 
//
// After completion of the traversal the control channel is closed.
func MutateWalk(pkg *types.Package, info *types.Info, node ast.Node, m mutator.Mutator) chan bool {
	w := &mutateWalk{
		changed: make(chan bool),
		mutator: m,
		pkg:     pkg,
		info:    info,
	}

	go func() {
		// runs `mutateWalk`s `Visit` function on each node as implemented below
		ast.Walk(w, node)

		close(w.changed)
	}()

	return w.changed
}

type mutateWalk struct {
	changed chan bool
	mutator mutator.Mutator
	pkg     *types.Package
	info    *types.Info
}

// Visit implements the Visit method of the ast.Visitor interface
func (w *mutateWalk) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return w
	}

	// loops through all possible mutations for node and runs them
	for _, m := range w.mutator(w.pkg, w.info, node) {
		// runs mutation logic on current node
		m.Change()
		w.changed <- true
		<-w.changed

		// resets node to original state
		m.Reset()
		w.changed <- true
		<-w.changed
	}

	return w
}

// PrintWalk traverses the AST of the given node and prints every node to STDOUT.
func PrintWalk(node ast.Node) {
	w := &printWalk{
		level: 0,
	}

	ast.Walk(w, node)
}

type printWalk struct {
	level int
}

// Visit implements the Visit method of the ast.Visitor interface
func (w *printWalk) Visit(node ast.Node) ast.Visitor {
	if node != nil {
		w.level++

		fmt.Printf("%s(%p)%#v\n", strings.Repeat("\t", w.level), node, node)
	} else {
		w.level--
	}

	return w
}

// Gets full AST of input node and its children and returns a string parsed version
func GetNodeASTString(node ast.Node) string {
	// save original stdOut
	oldOut := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	mutesting.PrintWalk(node)

	outC := make(chan string)
	// move output to a separate goroutine to not block stdOut indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	// restore original state
	w.Close()
	os.Stdout = oldOut
	out := <-outC

	return out
}
