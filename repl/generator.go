package zygo

import (
	"errors"
	"fmt"
)

var NoExpressionsFound = errors.New("No expressions found")

type Generator struct {
	env          *Glisp
	funcname     string
	tail         bool
	scopes       int
	instructions []Instruction
}

type Loop struct {
	stmtname       SexpSymbol
	loopStart      int
	loopLen        int
	breakOffset    int // i.e. relative to loopStart
	continueOffset int // i.e. relative to loopStart
}

func (loop *Loop) IsStackElem() {}

func NewGenerator(env *Glisp) *Generator {
	gen := new(Generator)
	gen.env = env
	gen.instructions = make([]Instruction, 0)
	// tail marks whether or not we are in the tail position
	gen.tail = false
	// scopes is the number of extra (non-function) scopes we've created
	gen.scopes = 0
	return gen
}

func (gen *Generator) AddInstructions(instr []Instruction) {
	gen.instructions = append(gen.instructions, instr...)
}

func (gen *Generator) AddInstruction(instr Instruction) {
	gen.instructions = append(gen.instructions, instr)
}

func (gen *Generator) GenerateBegin(expressions []Sexp) error {
	size := len(expressions)
	oldtail := gen.tail
	gen.tail = false
	if size == 0 {
		return NoExpressionsFound
	}
	for _, expr := range expressions[:size-1] {
		err := gen.Generate(expr)
		if err != nil {
			return err
		}
		// insert pops after all but the last instruction
		// that way the stack remains clean
		gen.AddInstruction(PopInstr(0))
	}
	gen.tail = oldtail
	return gen.Generate(expressions[size-1])
}

func buildSexpFun(
	env *Glisp,
	name string,
	funcargs SexpArray,
	funcbody []Sexp,
	orig Sexp) (SexpFunction, error) {

	gen := NewGenerator(env)
	gen.tail = true

	if len(name) == 0 {
		gen.funcname = env.GenSymbol("__anon").name
	} else {
		gen.funcname = name
	}

	argsyms := make([]SexpSymbol, len(funcargs))

	for i, expr := range funcargs {
		switch t := expr.(type) {
		case SexpSymbol:
			argsyms[i] = t
		default:
			return MissingFunction,
				errors.New("function argument must be symbol")
		}
	}

	varargs := false
	nargs := len(funcargs)

	if len(argsyms) >= 2 && argsyms[len(argsyms)-2].name == "&" {
		argsyms[len(argsyms)-2] = argsyms[len(argsyms)-1]
		argsyms = argsyms[0 : len(argsyms)-1]
		varargs = true
		nargs = len(argsyms) - 1
	}

	for i := len(argsyms) - 1; i >= 0; i-- {
		gen.AddInstruction(PopStackPutEnvInstr{argsyms[i]})
	}
	err := gen.GenerateBegin(funcbody)
	if err != nil {
		return MissingFunction, err
	}
	gen.AddInstruction(ReturnInstr{nil})

	newfunc := GlispFunction(gen.instructions)
	return MakeFunction(gen.funcname, nargs, varargs, newfunc, orig), nil
}

func (gen *Generator) GenerateFn(args []Sexp, orig Sexp) error {
	if len(args) < 2 {
		return errors.New("malformed function definition")
	}

	var funcargs SexpArray

	switch expr := args[0].(type) {
	case SexpArray:
		funcargs = expr
	default:
		return errors.New("function arguments must be in vector")
	}

	funcbody := args[1:]
	sfun, err := buildSexpFun(gen.env, "", funcargs, funcbody, orig)
	if err != nil {
		return err
	}
	gen.AddInstruction(PushInstrClosure{sfun})

	return nil
}

func (gen *Generator) GenerateDef(args []Sexp) error {
	if len(args) != 2 {
		return errors.New("Wrong number of arguments to def")
	}

	var sym SexpSymbol
	switch expr := args[0].(type) {
	case SexpSymbol:
		sym = expr
	default:
		return errors.New("Definition name must by symbol")
	}

	if gen.env.HasMacro(sym) {
		return fmt.Errorf("Already have macro named '%s': refusing "+
			"to define variable of same name.", sym.name)
	}

	gen.tail = false
	err := gen.Generate(args[1])
	if err != nil {
		return err
	}
	// duplicate the value so def leaves its value
	// on the stack and becomes an expression rather
	// than a statement.
	gen.AddInstruction(DupInstr(0))
	gen.AddInstruction(PopStackPutEnvInstr{sym})
	return nil
}

func (gen *Generator) GenerateDefn(args []Sexp, orig Sexp) error {
	if len(args) < 3 {
		return errors.New("Wrong number of arguments to defn")
	}

	var funcargs SexpArray
	switch expr := args[1].(type) {
	case SexpArray:
		funcargs = expr
	default:
		return errors.New("function arguments must be in vector")
	}

	var sym SexpSymbol
	switch expr := args[0].(type) {
	case SexpSymbol:
		sym = expr
	default:
		return errors.New("Definition name must by symbol")
	}
	if gen.env.HasMacro(sym) {
		return fmt.Errorf("Already have macro named '%s': refusing"+
			" to define function of same name.", sym.name)
	}

	sfun, err := buildSexpFun(gen.env, sym.name, funcargs, args[2:], orig)
	if err != nil {
		return err
	}

	gen.AddInstruction(PushInstr{sfun})
	gen.AddInstruction(PopStackPutEnvInstr{sym})
	gen.AddInstruction(PushInstr{SexpNull})

	return nil
}

func (gen *Generator) GenerateDefmac(args []Sexp, orig Sexp) error {
	if len(args) < 3 {
		return errors.New("Wrong number of arguments to defmac")
	}

	var funcargs SexpArray
	switch expr := args[1].(type) {
	case SexpArray:
		funcargs = expr
	default:
		return errors.New("function arguments must be in vector")
	}

	var sym SexpSymbol
	switch expr := args[0].(type) {
	case SexpSymbol:
		sym = expr
	default:
		return errors.New("Definition name must by symbol")
	}

	sfun, err := buildSexpFun(gen.env, sym.name, funcargs, args[2:], orig)
	if err != nil {
		return err
	}

	gen.env.macros[sym.number] = sfun
	gen.AddInstruction(PushInstr{SexpNull})

	return nil
}

func (gen *Generator) GenerateMacexpand(args []Sexp) error {
	if len(args) != 1 {
		return WrongNargs
	}

	var list SexpPair
	var islist bool
	var ismacrocall bool

	switch t := args[0].(type) {
	case SexpPair:
		if IsList(t.tail) {
			list = t
			islist = true
		}
	default:
		islist = false
	}

	var macro SexpFunction
	if islist {
		switch t := list.head.(type) {
		case SexpSymbol:
			macro, ismacrocall = gen.env.macros[t.number]
		default:
			ismacrocall = false
		}
	}

	if !ismacrocall {
		gen.AddInstruction(PushInstr{args[0]})
		return nil
	}

	macargs, err := ListToArray(list.tail)
	if err != nil {
		return err
	}

	// don't mess up the previous environment
	// just to run a macroexpand.
	newenv := gen.env.Duplicate()

	expr, err := newenv.Apply(macro, macargs)
	if err != nil {
		return err
	}
	quotedExpansion := Cons(gen.env.MakeSymbol("quote"), expr)

	gen.AddInstruction(PushInstr{quotedExpansion})
	return nil
}

func (gen *Generator) GenerateShortCircuit(or bool, args []Sexp) error {
	size := len(args)

	subgen := NewGenerator(gen.env)
	subgen.scopes = gen.scopes
	subgen.tail = gen.tail
	subgen.funcname = gen.funcname
	subgen.Generate(args[size-1])
	instructions := subgen.instructions

	for i := size - 2; i >= 0; i-- {
		subgen = NewGenerator(gen.env)
		subgen.Generate(args[i])
		subgen.AddInstruction(DupInstr(0))
		subgen.AddInstruction(BranchInstr{or, len(instructions) + 2})
		subgen.AddInstruction(PopInstr(0))
		instructions = append(subgen.instructions, instructions...)
	}
	gen.AddInstructions(instructions)

	return nil
}

func (gen *Generator) GenerateCond(args []Sexp) error {
	if len(args)%2 == 0 {
		return errors.New("missing default case")
	}

	subgen := NewGenerator(gen.env)
	subgen.tail = gen.tail
	subgen.scopes = gen.scopes
	subgen.funcname = gen.funcname
	err := subgen.Generate(args[len(args)-1])
	if err != nil {
		return err
	}
	instructions := subgen.instructions

	for i := len(args)/2 - 1; i >= 0; i-- {
		subgen.Reset()
		err := subgen.Generate(args[2*i])
		if err != nil {
			return err
		}
		pred_code := subgen.instructions

		subgen.Reset()
		subgen.tail = gen.tail
		subgen.scopes = gen.scopes
		subgen.funcname = gen.funcname
		err = subgen.Generate(args[2*i+1])
		if err != nil {
			return err
		}
		body_code := subgen.instructions

		subgen.Reset()
		subgen.AddInstructions(pred_code)
		subgen.AddInstruction(BranchInstr{false, len(body_code) + 2})
		subgen.AddInstructions(body_code)
		subgen.AddInstruction(JumpInstr{addpc: len(instructions) + 1})
		subgen.AddInstructions(instructions)
		instructions = subgen.instructions
	}

	gen.AddInstructions(instructions)
	return nil
}

func (gen *Generator) GenerateQuote(args []Sexp) error {
	for _, expr := range args {
		gen.AddInstruction(PushInstr{expr})
	}
	return nil
}

func (gen *Generator) GenerateLet(name string, args []Sexp) error {
	if len(args) < 2 {
		return errors.New("malformed let statement")
	}

	lstatements := make([]SexpSymbol, 0)
	rstatements := make([]Sexp, 0)
	var bindings []Sexp

	switch expr := args[0].(type) {
	case SexpArray:
		bindings = []Sexp(expr)
	default:
		return errors.New("let bindings must be in array")
	}

	if len(bindings)%2 != 0 {
		return errors.New("uneven let binding list")
	}

	for i := 0; i < len(bindings)/2; i++ {
		switch t := bindings[2*i].(type) {
		case SexpSymbol:
			lstatements = append(lstatements, t)
		default:
			return errors.New("cannot bind to non-symbol")
		}
		rstatements = append(rstatements, bindings[2*i+1])
	}

	gen.AddInstruction(AddScopeInstr(0))
	gen.scopes++

	if name == "let*" {
		for i, rs := range rstatements {
			err := gen.Generate(rs)
			if err != nil {
				return err
			}
			gen.AddInstruction(PopStackPutEnvInstr{lstatements[i]})
		}
	} else if name == "let" {
		for _, rs := range rstatements {
			err := gen.Generate(rs)
			if err != nil {
				return err
			}
		}
		for i := len(lstatements) - 1; i >= 0; i-- {
			gen.AddInstruction(PopStackPutEnvInstr{lstatements[i]})
		}
	}
	err := gen.GenerateBegin(args[1:])
	if err != nil {
		return err
	}
	gen.AddInstruction(RemoveScopeInstr(0))
	gen.scopes--

	return nil
}

func (gen *Generator) GenerateAssert(args []Sexp) error {
	if len(args) != 1 {
		return WrongNargs
	}
	err := gen.Generate(args[0])
	if err != nil {
		return err
	}

	reterrmsg := fmt.Sprintf("Assertion failed: %s\n",
		args[0].SexpString())
	gen.AddInstruction(BranchInstr{true, 2})
	gen.AddInstruction(ReturnInstr{errors.New(reterrmsg)})
	gen.AddInstruction(PushInstr{SexpNull})
	return nil
}

func (gen *Generator) GenerateInclude(args []Sexp) error {
	if len(args) < 1 {
		return WrongNargs
	}

	var err error
	var exps []Sexp

	var sourceItem func(item Sexp) error

	sourceItem = func(item Sexp) error {
		switch t := item.(type) {
		case SexpArray:
			for _, v := range t {
				if err := sourceItem(v); err != nil {
					return err
				}
			}
		case SexpPair:
			expr := item
			for expr != SexpNull {
				list := expr.(SexpPair)
				if err := sourceItem(list.head); err != nil {
					return err
				}
				expr = list.tail
			}
		case SexpStr:
			exps, err = gen.env.ParseFile(string(t))
			if err != nil {
				return err
			}

			err = gen.GenerateBegin(exps)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("include: Expected `string`, `list`, `array` given type %T val %v", item, item)
		}

		return nil
	}

	for _, v := range args {
		err = sourceItem(v)
		if err != nil {
			return err
		}
	}

	return nil
}

func (gen *Generator) GenerateCallBySymbol(sym SexpSymbol, args []Sexp, orig Sexp) error {
	switch sym.name {
	case "and":
		return gen.GenerateShortCircuit(false, args)
	case "or":
		return gen.GenerateShortCircuit(true, args)
	case "cond":
		return gen.GenerateCond(args)
	case "quote":
		return gen.GenerateQuote(args)
	case "setq":
		fallthrough
	case "def":
		return gen.GenerateDef(args)
	case "mdef":
		return gen.GenerateMultiDef(args)
	case "fn":
		return gen.GenerateFn(args, orig)
	case "defn":
		return gen.GenerateDefn(args, orig)
	case "begin":
		return gen.GenerateBegin(args)
	case "let":
		return gen.GenerateLet("let", args)
	case "let*":
		return gen.GenerateLet("let*", args)
	case "assert":
		return gen.GenerateAssert(args)
	case "defmac":
		return gen.GenerateDefmac(args, orig)
	case "macexpand":
		return gen.GenerateMacexpand(args)
	case "syntax-quote":
		return gen.GenerateSyntaxQuote(args)
	case "include":
		return gen.GenerateInclude(args)
	case "for":
		return gen.GenerateForLoop(args)
	case "set":
		return gen.GenerateSet(args)
	case "break":
		return gen.GenerateBreak(args)
	case "continue":
		return gen.GenerateContinue(args)
	}

	// this is where macros are run
	macro, found := gen.env.macros[sym.number]
	if found {
		// calling Apply on the current environment will screw up
		// the stack, creating a duplicate environment is safer
		env := gen.env.Duplicate()
		expr, err := env.Apply(macro, args)
		if err != nil {
			return err
		}
		return gen.Generate(expr)
	}

	oldtail := gen.tail
	gen.tail = false
	err := gen.GenerateAll(args)
	if err != nil {
		return err
	}
	if oldtail && sym.name == gen.funcname {
		// to do a tail call
		// pop off all the extra scopes
		// then jump to beginning of function
		for i := 0; i < gen.scopes; i++ {
			gen.AddInstruction(RemoveScopeInstr(0))
		}
		gen.AddInstruction(GotoInstr{0})
	} else {
		gen.AddInstruction(CallInstr{sym, len(args)})
	}
	gen.tail = oldtail
	return nil
}

func (gen *Generator) GenerateDispatch(fun Sexp, args []Sexp) error {
	gen.GenerateAll(args)
	gen.Generate(fun)
	gen.AddInstruction(DispatchInstr{len(args)})
	return nil
}

func (gen *Generator) GenerateCall(expr SexpPair) error {
	arr, _ := ListToArray(expr.tail)
	switch head := expr.head.(type) {
	case SexpSymbol:
		return gen.GenerateCallBySymbol(head, arr, expr)
	}
	return gen.GenerateDispatch(expr.head, arr)
}

func (gen *Generator) GenerateArray(arr SexpArray) error {
	err := gen.GenerateAll(arr)
	if err != nil {
		return err
	}
	gen.AddInstruction(CallInstr{gen.env.MakeSymbol("array"), len(arr)})
	return nil
}

func (gen *Generator) Generate(expr Sexp) error {
	switch e := expr.(type) {
	case SexpSymbol:
		gen.AddInstruction(EnvToStackInstr{e})
		return nil
	case SexpPair:
		if IsList(e) {
			err := gen.GenerateCall(e)
			if err != nil {
				return errors.New(
					fmt.Sprintf("Error generating %s:\n%v",
						expr.SexpString(), err))
			}
			return nil
		} else {
			gen.AddInstruction(PushInstr{expr})
		}
	case SexpArray:
		return gen.GenerateArray(e)
	default:
		gen.AddInstruction(PushInstr{expr})
		return nil
	}
	return nil
}

func (gen *Generator) GenerateAll(expressions []Sexp) error {
	for _, expr := range expressions {
		err := gen.Generate(expr)
		if err != nil {
			return err
		}
	}
	return nil
}

func (gen *Generator) Reset() {
	gen.instructions = make([]Instruction, 0)
	gen.tail = false
	gen.scopes = 0
}

// for loops: Just like in C.
//
// (for [init predicate advance] (expr)*)
//
// Each of init, predicate, and advance are expressions
// that are evaluated during the running of the for loop.
// The init expression is evaluated once at the top.
// The predicate is tested. If it is true then
// the body expressions are run. Then the advance
// expression is evaluated, and we return to the
// predicate test.
func (gen *Generator) GenerateForLoop(args []Sexp) error {
	narg := len(args)
	if narg < 2 {
		return errors.New("malformed for loop")
	}

	var controlargs SexpArray
	switch expr := args[0].(type) {
	case SexpArray:
		controlargs = expr
	default:
		return errors.New("for loop: first argument must be a vector of [init predicate advance]")
	}

	if len(controlargs) != 3 {
		return errors.New("for loop: first argument wrong size; must be a vector of three [init test advance]")
	}

	loop := &Loop{
		stmtname: gen.env.GenSymbol("__loop"),
	}
	gen.env.loopstack.Push(loop)
	defer gen.env.loopstack.Pop()

	startPos := len(gen.instructions)

	gen.AddInstruction(LoopStartInstr{loop: loop})
	gen.AddInstruction(PushStackmarkInstr{sym: loop.stmtname})

	// generate the body of the loop
	subgenBody := NewGenerator(gen.env)
	subgenBody.tail = gen.tail
	subgenBody.scopes = gen.scopes
	subgenBody.funcname = gen.funcname
	err := subgenBody.GenerateBegin(args[1:])
	if err != nil {
		return err
	}
	// insert pop so the stack remains clean
	subgenBody.AddInstruction(PopUntilStackmarkInstr{sym: loop.stmtname})
	len_body_code := len(subgenBody.instructions)

	// generate the init code
	subgenInit := NewGenerator(gen.env)
	subgenInit.tail = gen.tail
	subgenInit.scopes = gen.scopes
	subgenInit.funcname = gen.funcname
	err = subgenInit.Generate(controlargs[0])
	if err != nil {
		return err
	}
	// insert pop so the stack remains clean
	subgenInit.AddInstruction(PopUntilStackmarkInstr{sym: loop.stmtname})
	init_code := subgenInit.instructions

	// generate the test
	subgenT := NewGenerator(gen.env)
	subgenT.tail = gen.tail
	subgenT.scopes = gen.scopes
	subgenT.funcname = gen.funcname
	err = subgenT.Generate(controlargs[1])
	if err != nil {
		return err
	}
	// need to leave value on stack to branch on
	// so do not popuntil stackmark here!
	test_code := subgenT.instructions

	// generate the increment code
	subgenIncr := NewGenerator(gen.env)
	subgenIncr.tail = gen.tail
	subgenIncr.scopes = gen.scopes
	subgenIncr.funcname = gen.funcname
	err = subgenIncr.Generate(controlargs[2])
	if err != nil {
		return err
	}
	subgenIncr.AddInstruction(PopUntilStackmarkInstr{sym: loop.stmtname})
	incr_code := subgenIncr.instructions

	// Don't add a new scope for a for-loop.
	// A new scope would make it hard to update variables
	// just outside the loop using (def). If we
	// had to use (set) it would be dangerous and
	// more error prone.

	exit_loop := len_body_code + 3
	jump_to_test := len(incr_code) + 2

	gen.AddInstruction(LabelInstr{label: "start of init for " + loop.stmtname.name})
	gen.AddInstructions(init_code)
	gen.AddInstruction(JumpInstr{addpc: jump_to_test, where: "to-test"})
	// top of loop starts with test_code: (continue) target.
	continuePos := len(gen.instructions)
	gen.AddInstruction(LabelInstr{label: "start of increment for " + loop.stmtname.name})
	gen.AddInstructions(incr_code)
	gen.AddInstruction(LabelInstr{label: "start of test for " + loop.stmtname.name})
	gen.AddInstructions(test_code)
	gen.AddInstruction(BranchInstr{false, exit_loop})
	bodyPos := len(gen.instructions)

	// Set the break/continue offsets.
	//
	// The final coordinates are relative to gen, but the
	// break statement will only know its own position with
	// respect to subgenBody, so we use loopStart to tell it
	// the additional (negative) distance to startPos.

	gen.AddInstruction(LabelInstr{label: "start of body for " + loop.stmtname.name})
	gen.AddInstructions(subgenBody.instructions)
	gen.AddInstruction(JumpInstr{addpc: continuePos - len(gen.instructions),
		where: "to-continue-position-aka-increment"})
	gen.AddInstruction(LabelInstr{label: "end of body for " + loop.stmtname.name})
	// bottom is (break) target

	// cleanup
	gen.AddInstruction(ClearStackmarkInstr{sym: loop.stmtname})
	gen.AddInstruction(PushInstr{SexpNull}) // for is a statement; leave null on the stack.

	loop.loopStart = startPos - bodyPos // offset; should be negative.
	loop.loopLen = len(gen.instructions) - startPos
	loop.breakOffset = loop.loopLen
	loop.continueOffset = continuePos - startPos

	VPrintf("\n debug at end of for loop generation, loop = %#v at address %p\n",
		loop, loop)

	return nil
}

func (gen *Generator) GenerateSet(args []Sexp) error {
	narg := len(args)
	if narg != 2 {
		return errors.New("malformed set statement, need 2 arguments")
	}

	var lhs SexpSymbol
	switch expr := args[0].(type) {
	case SexpSymbol:
		lhs = expr
	default:
		return errors.New("set: left-hand-side must be a symbol")
	}

	if gen.env.HasMacro(lhs) {
		return fmt.Errorf("Already have macro named '%s': refusing "+
			"to set variable of same name.", lhs.name)
	}

	rhs := args[1]
	gen.tail = false
	err := gen.Generate(rhs)
	if err != nil {
		return err
	}

	// leaving a copy on the stack makes set an expression
	// with a value. Useful for chaining.
	gen.AddInstruction(DupInstr(0))
	gen.AddInstruction(UpdateInstr{lhs})
	return nil

}

// (mdef a b c (list 1 2 3)) will bind a:1 b:2 c:3
func (gen *Generator) GenerateMultiDef(args []Sexp) error {
	if len(args) < 2 {
		return errors.New("Wrong number of arguments to def")
	}

	nsym := len(args) - 1
	lastpos := len(args) - 1
	syms := make([]SexpSymbol, nsym)
	for i := 0; i < nsym; i++ {
		switch sym := args[i].(type) {
		case SexpSymbol:
			syms[i] = sym
			if gen.env.HasMacro(sym) {
				return fmt.Errorf("Already have macro named '%s': refusing "+
					"to define variable of same name.", sym.name)
			}
		case SexpPair:
			// gracefully handle the quoted symbols we get from the range macro
			unquotedSymbol, isQuo := isQuotedSymbol(sym)
			if isQuo {
				syms[i] = unquotedSymbol.(SexpSymbol)
			}
		default:
			return fmt.Errorf("All mdef targets must be symbols, but %d-th was not, instead of type %T: '%s'", i+1, sym, sym.SexpString())
		}
	}

	gen.tail = false
	err := gen.Generate(args[lastpos])
	if err != nil {
		return err
	}
	// duplicate the value so def leaves its value
	// on the stack and becomes an expression rather
	// than a statement.
	gen.AddInstruction(DupInstr(0))
	gen.AddInstruction(BindlistInstr{syms: syms})
	return nil
}

func isQuotedSymbol(list SexpPair) (unquotedSymbol Sexp, isQuo bool) {
	head := list.head
	switch h := head.(type) {
	case SexpSymbol:
		if h.name != "quote" {
			return SexpNull, false
		}
	}
	// good, keep going to tail
	t := list.tail
	switch tt := t.(type) {
	case SexpPair:
		// good, keep going to head
		hh := tt.head
		switch hhh := hh.(type) {
		case SexpSymbol:
			// grab the symbol
			return hhh, true
		}
	}
	return SexpNull, false
}

// side-effect (or main effect) has to be pushing an expression on the top of
// the datastack that represents the expanded and substituted expression
func (gen *Generator) GenerateSyntaxQuote(args []Sexp) error {
	VPrintf("\n GenerateSyntaxQuote() called with args[0]='%#v'\n", args[0])

	if len(args) != 1 {
		return errors.New("syntax-quote takes exactly one argument")
	}
	arg := args[0]

	// need to handle arrays, since they can have unquotes
	// in them too.
	switch arg.(type) {
	case SexpArray:
		gen.generateSyntaxQuoteArray(arg)
		return nil
	case SexpPair:
		if !IsList(arg) {
			break
		}
		gen.generateSyntaxQuoteList(arg)
		return nil
	case SexpHash:
		gen.generateSyntaxQuoteHash(arg)
		return nil
	}
	gen.AddInstruction(PushInstr{arg})
	return nil
}

func (gen *Generator) generateSyntaxQuoteList(arg Sexp) error {
	VPrintf("\n GenerateSyntaxQuoteList() called with arg='%#v'\n", arg)

	switch a := arg.(type) {
	case SexpPair:
		//good, required here
	default:
		return fmt.Errorf("arg to generateSyntaxQuoteList() must be list; got %T", a)
	}

	// things that need unquoting end up as
	// (unquote mysym)
	// i.e. a pair
	// list of length 2 exactly, with first atom
	// being "unquote" and second being the symbol
	// to substitute.
	quotebody, _ := ListToArray(arg)
	VPrintf("\n quotebody = '%#v'\n", quotebody)

	if len(quotebody) == 2 {
		var issymbol bool
		var sym SexpSymbol
		switch t := quotebody[0].(type) {
		case SexpSymbol:
			sym = t
			issymbol = true
		default:
			issymbol = false
		}
		if issymbol {
			if sym.name == "unquote" {
				VPrintf("detected unquote with quotebody[1]='%#v'   arg='%#v'\n", quotebody[1], arg)
				gen.Generate(quotebody[1])
				return nil
			} else if sym.name == "unquote-splicing" {
				gen.Generate(quotebody[1])
				gen.AddInstruction(ExplodeInstr(0))
				return nil
			}
		}
	}

	gen.AddInstruction(PushInstr{SexpMarker})

	for _, expr := range quotebody {
		gen.GenerateSyntaxQuote([]Sexp{expr})
	}

	gen.AddInstruction(SquashInstr(0))

	return nil
}

func (gen *Generator) generateSyntaxQuoteArray(arg Sexp) error {
	VPrintf("\n GenerateSyntaxQuoteArray() called with arg='%#v'\n", arg)

	var arr SexpArray
	switch a := arg.(type) {
	case SexpArray:
		//good, required here
		arr = a
	default:
		return fmt.Errorf("arg to generateSyntaxQuoteArray() must be an array; got %T", a)
	}

	gen.AddInstruction(PushInstr{SexpMarker})
	for _, expr := range arr {
		gen.AddInstruction(PushInstr{SexpMarker})
		gen.GenerateSyntaxQuote([]Sexp{expr})
		gen.AddInstruction(SquashInstr(0))
		gen.AddInstruction(ExplodeInstr(0))
	}
	gen.AddInstruction(VectorizeInstr(0))
	return nil
}

func (gen *Generator) generateSyntaxQuoteHash(arg Sexp) error {
	VPrintf("\n GenerateSyntaxQuoteHash() called with arg='%#v'\n", arg)

	var hash SexpHash
	switch a := arg.(type) {
	case SexpHash:
		//good, required here
		hash = a
	default:
		return fmt.Errorf("arg to generateSyntaxQuoteHash() must be a hash; got %T", a)
	}
	n := HashCountKeys(hash)
	gen.AddInstruction(PushInstr{SexpMarker})
	for i := 0; i < n; i++ {
		// must reverse order here to preserve order on rebuild
		key := (*hash.KeyOrder)[(n-i)-1]
		val, err := hash.HashGet(key)
		if err != nil {
			return err
		}
		// value first, since value comes second on rebuild
		gen.AddInstruction(PushInstr{SexpMarker})
		gen.GenerateSyntaxQuote([]Sexp{val})
		gen.AddInstruction(SquashInstr(0))
		gen.AddInstruction(ExplodeInstr(0))

		gen.AddInstruction(PushInstr{SexpMarker})
		gen.GenerateSyntaxQuote([]Sexp{key})
		gen.AddInstruction(SquashInstr(0))
		gen.AddInstruction(ExplodeInstr(0))
	}
	gen.AddInstruction(HashizeInstr{
		HashLen:  n,
		TypeName: *(hash.TypeName),
	})
	return nil
}

func (gen *Generator) GenerateContinue(args []Sexp) error {
	if len(args) != 0 {
		return fmt.Errorf("(continue) takes no arguments")
	}
	lse, err := gen.env.loopstack.Get(gen.env.loopstack.Top())
	if err != nil || lse == nil {
		return fmt.Errorf("(continue) found but not inside a loop.")
	}
	var loop *Loop
	switch e := lse.(type) {
	case *Loop:
		loop = e
	default:
		panic(fmt.Errorf("unexpected type found on loopstack: type=%T  value='%#v'", e, e))
	}

	myPos := len(gen.instructions)
	VPrintf("\n debug GenerateContinue() : myPos =%d  loop=%#v\n", myPos, loop)
	gen.AddInstruction(&ContinueInstr{loop: loop})
	return nil
}

func (gen *Generator) GenerateBreak(args []Sexp) error {
	if len(args) != 0 {
		return fmt.Errorf("(break) takes no arguments")
	}

	lse, err := gen.env.loopstack.Get(gen.env.loopstack.Top())
	if err != nil || lse == nil {
		return fmt.Errorf("(break) found but not inside a loop.")
	}

	var loop *Loop
	switch e := lse.(type) {
	case *Loop:
		loop = e
	default:
		panic(fmt.Errorf("unexpected type found on loopstack: type=%T  value='%#v'", e, e))
	}

	VPrintf("\n debug GenerateBreak() : loop=%#v\n", loop)
	gen.AddInstruction(&BreakInstr{loop: loop})

	return nil
}
