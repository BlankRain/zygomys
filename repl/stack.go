package zygo

import (
	"errors"
	"fmt"
	"strings"
)

type StackElem interface {
	IsStackElem()
}

type Stack struct {
	tos      int
	elements []StackElem
	env      *Glisp
}

func (env *Glisp) NewStack(size int) *Stack {
	return &Stack{
		tos:      -1,
		elements: make([]StackElem, size),
		env:      env,
	}
}

func (stack *Stack) Clone() *Stack {
	ret := &Stack{}
	ret.tos = stack.tos
	ret.elements = make([]StackElem, len(stack.elements))
	for i := range stack.elements {
		ret.elements[i] = stack.elements[i]
	}

	return ret
}

func (stack *Stack) Top() int {
	return stack.tos
}

func (stack *Stack) PushAllTo(target *Stack) int {
	if stack.tos < 0 {
		return 0
	}

	for _, v := range stack.elements[0 : stack.tos+1] {
		target.Push(v)
	}

	return stack.tos + 1
}

func (stack *Stack) IsEmpty() bool {
	return stack.tos < 0
}

func (stack *Stack) Push(elem StackElem) {
	stack.tos++

	if stack.tos == len(stack.elements) {
		stack.elements = append(stack.elements, elem)
	} else {
		stack.elements[stack.tos] = elem
	}
}

func (stack *Stack) Get(n int) (StackElem, error) {
	if stack.tos-n < 0 {
		return nil, errors.New(fmt.Sprint("invalid stack access asked for ", n, " Top was ", stack.tos))
	}
	return stack.elements[stack.tos-n], nil
}

func (stack *Stack) Pop() (StackElem, error) {
	elem, err := stack.Get(0)
	if err != nil {
		return nil, err
	}
	stack.tos--
	return elem, nil
}

func (stack *Stack) IsStackElem() {}

func (stack Stack) Show(env *Glisp, indent int, label string) error {
	rep := strings.Repeat(" ", indent)
	fmt.Printf("%s %s\n", rep, label)
	n := stack.Top()
	for i := 0; i <= n; i++ {
		ele, err := stack.Get(n - i)
		if err != nil {
			panic(fmt.Errorf("stack access error on %i: %v", i, err))
		}
		showme, canshow := ele.(Showable)
		if canshow {
			showme.Show(env, indent+4, fmt.Sprintf("elem %v of %s:", i, label))
		}
	}
	return nil
}
