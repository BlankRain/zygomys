package zygo

import "fmt"

func MapArray(env *Glisp, fun *SexpFunction, arr *SexpArray) (Sexp, error) {
	result := make([]Sexp, len(arr.Val))
	var err error

	var firstTyp *RegisteredType
	for i := range arr.Val {
		result[i], err = env.Apply(fun, arr.Val[i:i+1])
		if err != nil {
			return &SexpArray{Val: result, Typ: firstTyp}, err
		}
		if firstTyp == nil {
			firstTyp = result[i].Type()
		}
	}

	return &SexpArray{Val: result, Typ: firstTyp}, nil
}

func ConcatArray(arr *SexpArray, rest []Sexp) (Sexp, error) {
	if arr == nil {
		return SexpNull, fmt.Errorf("ConcatArray called with nil arr")
	}
	var res SexpArray
	res.Val = arr.Val
	for i, x := range rest {
		switch t := x.(type) {
		case *SexpArray:
			res.Val = append(res.Val, t.Val...)
		default:
			return &res, fmt.Errorf("ConcatArray error: %d-th argument "+
				"(0-based) is not an array", i)
		}
	}
	return &res, nil
}

// (arrayidx ar [0 1])
func ArrayIndexFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	//P("in ArrayIndexFunction, args = '%#v'", args)
	narg := len(args)
	if narg != 2 {
		return SexpNull, WrongNargs
	}

	var err error
	args, err = env.ResolveDotSym(args)
	if err != nil {
		return SexpNull, err
	}

	ar, isAr := args[0].(*SexpArray)
	if !isAr {
		return SexpNull, fmt.Errorf("bad (arrayidx ar index) call: ar was not an array, instead '%s'/type %T",
			args[0].SexpString(0), args[0])
	}

	idx, isAr := args[1].(*SexpArray)
	if !isAr {
		return SexpNull, fmt.Errorf("bad (arrayidx ar index) call: index was not an array, instead '%s'/type %T",
			args[1].SexpString(0), args[1])
	}

	return ar.IndexBy(idx)
}

// IndexBy subsets one array (possibly multidimensional) by another.
// e.g. if arr is [a b c] and idx is [0], we'll return a.
func (arr *SexpArray) IndexBy(idx *SexpArray) (Sexp, error) {
	nIdx := len(idx.Val)
	nTarget := arr.NumDim()

	if nIdx > nTarget {
		return SexpNull, fmt.Errorf("bad (arrayidx ar index) call: index requested %d dimensions, only have %d",
			nIdx, nTarget)
	}

	if len(idx.Val) == 0 {
		return SexpNull, fmt.Errorf("bad (arrayidx ar index) call: no index supplied")
	}
	if len(idx.Val) != 1 {
		return SexpNull, fmt.Errorf("bad (arrayidx ar index) call: we only support a single index value atm")
	}

	i := 0
	myInt, isInt := idx.Val[i].(*SexpInt)
	if !isInt {
		return SexpNull, fmt.Errorf("bad (arrayidx ar index) call: index with non-integer '%v'",
			idx.Val[i].SexpString(0))
	}
	k := myInt.Val
	pos := k % int64(len(arr.Val))
	if k < 0 {
		mk := -k
		mod := mk % int64(len(arr.Val))
		pos = int64(len(arr.Val)) - mod
	}
	//P("return pos %v", pos)
	return arr.Val[pos], nil
}

func (arr *SexpArray) NumDim() int {
	return 1
}
