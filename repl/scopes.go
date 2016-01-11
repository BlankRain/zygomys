package zygo

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Scope map[int]Sexp

func (s Scope) IsStackElem() {}

func (stack *Stack) PushScope() {
	stack.Push(Scope(make(map[int]Sexp)))
}

func (stack *Stack) PopScope() error {
	_, err := stack.Pop()
	return err
}

func (stack *Stack) lookupSymbol(sym SexpSymbol, minFrame int) (Sexp, error, *Scope) {
	if !stack.IsEmpty() {
		for i := 0; i <= stack.tos-minFrame; i++ {
			elem, err := stack.Get(i)
			if err != nil {
				return SexpNull, err, nil
			}
			scope := map[int]Sexp(elem.(Scope))
			sc := Scope(scope)
			expr, ok := scope[sym.number]
			if ok {
				return expr, nil, &sc
			}
		}
	}
	return SexpNull, errors.New(fmt.Sprint("symbol ", sym, " not found")), nil
}

func (stack *Stack) LookupSymbol(sym SexpSymbol) (Sexp, error, *Scope) {
	return stack.lookupSymbol(sym, 0)
}

// LookupSymbolNonGlobal  - closures use this to only find symbols below the global scope, to avoid copying globals it'll always be-able to ref
func (stack *Stack) LookupSymbolNonGlobal(sym SexpSymbol) (Sexp, error, *Scope) {
	return stack.lookupSymbol(sym, 1)
}

func (stack *Stack) BindSymbol(sym SexpSymbol, expr Sexp) error {
	if stack.IsEmpty() {
		return errors.New("no scope available")
	}
	stack.elements[stack.tos].(Scope)[sym.number] = expr
	return nil
}

// used to implement (set v 10)
func (scope *Scope) UpdateSymbolInScope(sym SexpSymbol, expr Sexp) error {

	_, found := (*scope)[sym.number]
	if !found {
		return fmt.Errorf("symbol %s not found", sym.name)
	}
	(*scope)[sym.number] = expr
	return nil
}

type SymtabE struct {
	Key string
	Val string
}

type SymtabSorter []*SymtabE

func (a SymtabSorter) Len() int           { return len(a) }
func (a SymtabSorter) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a SymtabSorter) Less(i, j int) bool { return a[i].Key < a[j].Key }

func (s *Scope) Show(env *Glisp, indent int) {
	sortme := []*SymtabE{}
	for symbolNumber, val := range *s {
		symbolName := env.revsymtable[symbolNumber]
		sortme = append(sortme, &SymtabE{Key: symbolName, Val: val.SexpString()})
	}
	sort.Sort(SymtabSorter(sortme))
	rep := strings.Repeat(" ", indent)
	for i := range sortme {
		fmt.Printf("%s %s -> %s\n", rep,
			sortme[i].Key, sortme[i].Val)
	}
}
