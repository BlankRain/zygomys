package zygo

import (
	"fmt"
	"reflect"
)

// package.go: declare package, structs, function types

// A builder is a special kind of function. Like
// a macro it receives the un-evaluated tree
// of symbols from its caller. A builder
// can therefore be used to build new types
// and declarations new functions/methods.
//
// Like a function, a builder is called at
// run/evaluation time, not at definition time.
//
// The primary use here is to be able to define
// packges, structs, interfaces, functions,
// methods, and type aliases.
//
func (env *Glisp) ImportPackageBuilder() {
	env.AddBuilder("package", PackageBuilder)
	env.AddFunction("slice-of", SliceOfFunction)
	env.AddFunction("pointer-to", PointerToFunction)
	env.AddBuilder("interface", InterfaceBuilder)
	env.AddBuilder("func", FuncBuilder)
}

// this is just a stub. TODO: finish design, implement packages.
func PackageBuilder(env *Glisp, name string,
	args []Sexp) (Sexp, error) {

	if len(args) < 1 {
		return SexpNull, fmt.Errorf("package name is missing. use: " +
			"(package package-name ...)\n")
	}

	P("in package builder, args = ")
	for i := range args {
		P("args[%v] = '%s'", i, args[i].SexpString())
	}

	return SexpNull, nil
}

func InterfaceBuilder(env *Glisp, name string,
	args []Sexp) (Sexp, error) {

	if len(args) < 1 {
		return SexpNull, fmt.Errorf("interface name is missing. use: " +
			"(interface interface-name ...)\n")
	}

	P("in interface builder, args = ")
	for i := range args {
		P("args[%v] = '%s'", i, args[i].SexpString())
	}

	return SexpNull, nil
}

func FuncBuilder(env *Glisp, name string,
	args []Sexp) (Sexp, error) {

	if len(args) < 1 {
		return SexpNull, fmt.Errorf("func name is missing. use: " +
			"(func func-name ...)\n")
	}

	P("in func builder, args = ")
	for i := range args {
		P("args[%v] = '%s'", i, args[i].SexpString())
	}

	return SexpNull, nil
}

func SliceOfFunction(env *Glisp, name string,
	args []Sexp) (Sexp, error) {

	if len(args) != 1 {
		return SexpNull, fmt.Errorf("argument to slice-of is missing. use: " +
			"(slice-of a-regtype)\n")
	}

	var rt *RegisteredType
	switch arg := args[0].(type) {
	case *RegisteredType:
		rt = arg
	default:
		return SexpNull, fmt.Errorf("argument to slice-of was not regtype, "+
			"instead type %T displaying as '%v' ",
			arg, arg.SexpString())
	}

	//P("slice-of arg = '%s' with type %T", args[0].SexpString(), args[0])

	derivedType := reflect.SliceOf(rt.TypeCache)
	sliceRt := NewRegisteredType(func(env *Glisp) interface{} {
		return reflect.MakeSlice(derivedType, 0, 0)
	})
	sliceRt.DisplayAs = fmt.Sprintf("(slice-of %s)", rt.DisplayAs)
	sliceName := "slice-of-" + rt.RegisteredName
	GoStructRegistry.RegisterUserdef(sliceName, sliceRt)
	return sliceRt, nil
}

func PointerToFunction(env *Glisp, name string,
	args []Sexp) (Sexp, error) {

	if len(args) != 1 {
		return SexpNull, fmt.Errorf("argument to pointer-to is missing. use: " +
			"(pointer-to a-regtype)\n")
	}

	var rt *RegisteredType
	switch arg := args[0].(type) {
	case *RegisteredType:
		rt = arg
	default:
		return SexpNull, fmt.Errorf("argument to pointer-to was not regtype, "+
			"instead type %T displaying as '%v' ",
			arg, arg.SexpString())
	}

	//P("pointer-to arg = '%s' with type %T", args[0].SexpString(), args[0])

	derivedType := reflect.PtrTo(rt.TypeCache)
	sliceRt := NewRegisteredType(func(env *Glisp) interface{} {
		return reflect.New(derivedType)
	})
	sliceRt.DisplayAs = fmt.Sprintf("(pointer-to %s)", rt.DisplayAs)
	sliceName := "pointer-to-" + rt.RegisteredName
	GoStructRegistry.RegisterUserdef(sliceName, sliceRt)
	return sliceRt, nil
}
