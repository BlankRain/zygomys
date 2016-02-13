package zygo

import "fmt"

func IsArray(expr Sexp) bool {
	switch expr.(type) {
	case SexpArray:
		return true
	}
	return false
}

func IsList(expr Sexp) bool {
	if expr == SexpNull {
		return true
	}
	switch list := expr.(type) {
	case SexpPair:
		return IsList(list.Tail)
	}
	return false
}

func IsAssignmentList(expr Sexp, pos int) (bool, int) {
	if expr == SexpNull {
		return false, -1
	}
	switch list := expr.(type) {
	case SexpPair:
		sym, isSym := list.Head.(SexpSymbol)
		if !isSym {
			return IsAssignmentList(list.Tail, pos+1)
		}
		if sym.name == "=" || sym.name == ":=" {
			return true, pos
		}
		return IsAssignmentList(list.Tail, pos+1)
	}
	return false, -1
}

func IsFloat(expr Sexp) bool {
	switch expr.(type) {
	case SexpFloat:
		return true
	}
	return false
}

func IsInt(expr Sexp) bool {
	switch expr.(type) {
	case SexpInt:
		return true
	}
	return false
}

func IsString(expr Sexp) bool {
	switch expr.(type) {
	case SexpStr:
		return true
	}
	return false
}

func IsChar(expr Sexp) bool {
	switch expr.(type) {
	case SexpChar:
		return true
	}
	return false
}

func IsNumber(expr Sexp) bool {
	switch expr.(type) {
	case SexpFloat:
		return true
	case SexpInt:
		return true
	case SexpChar:
		return true
	}
	return false
}

func IsSymbol(expr Sexp) bool {
	switch expr.(type) {
	case SexpSymbol:
		return true
	}
	return false
}

func IsHash(expr Sexp) bool {
	switch expr.(type) {
	case *SexpHash:
		return true
	}
	return false
}

func IsZero(expr Sexp) bool {
	switch e := expr.(type) {
	case SexpInt:
		return int(e) == 0
	case SexpChar:
		return int(e) == 0
	case SexpFloat:
		return float64(e) == 0.0
	}
	return false
}

func IsEmpty(expr Sexp) bool {
	if expr == SexpNull {
		return true
	}

	switch e := expr.(type) {
	case SexpArray:
		return len(e) == 0
	case *SexpHash:
		return HashIsEmpty(e)
	}

	return false
}

func IsFunc(expr Sexp) bool {
	switch expr.(type) {
	case *SexpFunction:
		return true
	}
	return false
}

func TypeOf(expr Sexp) SexpStr {
	v := ""
	switch e := expr.(type) {
	case SexpRaw:
		v = "raw"
	case SexpArray:
		v = "array"
	case SexpInt:
		v = "int64"
	case SexpStr:
		v = "string"
	case SexpChar:
		v = "char"
	case SexpFloat:
		v = "float64"
	case *SexpHash:
		v = "hash"
	case SexpPair:
		v = "list"
	case SexpSymbol:
		v = "symbol"
	case *SexpFunction:
		v = "func"
	case SexpSentinel:
		v = "null"
	case SexpTime:
		v = "time.Time"
	default:
		fmt.Printf("\n error: unknown type: %T in '%#v'\n", e, e)
	}
	return SexpStr(v)
}
