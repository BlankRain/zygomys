package zygo

import (
	"fmt"
	"os/exec"
	"strings"
)

// system: ($) is macro. shell out, return the combined output.
func SystemFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) == 0 {
		return SexpNull, WrongNargs
	}

	flat, err := flattenToWordsHelper(args)
	if err != nil {
		return SexpNull, fmt.Errorf("flatten on '%#v' failed with error '%s'", args, err)
	}
	if len(flat) == 0 {
		return SexpNull, WrongNargs
	}

	cmd := flat[0]

	out, err := exec.Command(cmd, flat[1:]...).CombinedOutput()
	if err != nil {
		return SexpNull, err
	}
	return SexpStr(string(out)), nil
}

// given strings/lists of strings with possible whitespace
// flatten out to a array of SexpStr with no internal whitespace,
// suitable for passing along to (system) / exec.Command()
func FlattenToWordsFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) == 0 {
		return SexpNull, WrongNargs
	}
	stringArgs, err := flattenToWordsHelper(args)
	if err != nil {
		return SexpNull, err
	}

	// Now convert to []Sexp{SexpStr}
	res := make([]Sexp, len(stringArgs))
	for i := range stringArgs {
		res[i] = SexpStr(stringArgs[i])
	}
	return SexpArray(res), nil
}

func flattenToWordsHelper(args []Sexp) ([]string, error) {
	stringArgs := []string{}

	for i := range args {
		switch c := args[i].(type) {
		case SexpStr:
			many := strings.Split(string(c), " ")
			stringArgs = append(stringArgs, many...)
		case SexpSymbol:
			stringArgs = append(stringArgs, c.name)
		case SexpPair:
			carry, err := ListToArray(c)
			if err != nil {
				return []string{}, fmt.Errorf("tried to convert list of strings to array but failed with error '%s'. Input was type %T / val = '%#v'", c, c)
			}
			moreWords, err := flattenToWordsHelper(carry)
			if err != nil {
				return []string{}, err
			}
			stringArgs = append(stringArgs, moreWords...)
		default:
			return []string{}, fmt.Errorf("arguments to system must be strings; instead we have %T / val = '%#v'", c, c)
		}
	} // end i over args
	// INVAR: stringArgs has our flattened list.
	return stringArgs, nil
}
