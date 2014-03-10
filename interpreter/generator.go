package glisp

import (
	"errors"
)

type Generator struct {
	env *Glisp
	funcname string
	instructions []Instruction
}

func NewGenerator(env *Glisp) *Generator {
	gen := new(Generator)
	gen.env = env
	gen.instructions = make([]Instruction, 0)
	return gen
}

func (gen *Generator) AddInstructions(instr []Instruction) {
	gen.instructions = append(gen.instructions, instr...)
}

func (gen *Generator) AddInstruction(instr Instruction) {
	gen.instructions = append(gen.instructions, instr)
}

func (gen *Generator) GenerateFn(args []Sexp) error {
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
	subgen := NewGenerator(gen.env)

	for i := len(funcargs) - 1; i >= 0; i-- {
		var argsym SexpSymbol
		switch expr := funcargs[i].(type) {
		case SexpSymbol:
			argsym = expr
		default:
			return errors.New("function argument must be symbol")
		}
		subgen.AddInstruction(PutInstr{argsym})
	}
	err := subgen.GenerateAll(funcbody)
	if err != nil {
		return err
	}
	subgen.AddInstruction(ReturnInstr{nil})

	newfunc := GlispFunction(subgen.instructions)
	gen.AddInstruction(PushInstr{newfunc})

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

	err := gen.Generate(args[1])
	if err != nil {
		return err
	}
	gen.AddInstruction(PutInstr{sym})
	gen.AddInstruction(PushInstr{SexpNull})
	return nil
}

func (gen *Generator) GenerateDefn(args []Sexp) error {
	if len(args) < 3 {
		return errors.New("Wrong number of arguments to defn")
	}
	funcdef := MakeList(append([]Sexp{
		gen.env.MakeSymbol("fn"),
		args[1],
	}, args[2:]...))
	transformation := MakeList([]Sexp{
		gen.env.MakeSymbol("def"),
		args[0],
		funcdef,
	})
	return gen.Generate(transformation)
}

func (gen *Generator) GenerateShortCircuit(or bool, args []Sexp) error {
	size := len(args)

	subgen := NewGenerator(gen.env)
	subgen.Generate(args[size - 1])
	instructions := subgen.instructions

	for i := size - 2; i >= 0; i-- {
		subgen = NewGenerator(gen.env)
		subgen.Generate(args[i])
		branch := BranchInstr{or, len(instructions) + 1}
		instructions = append(
			subgen.instructions,
			append([]Instruction{branch}, instructions...)...)
	}
	gen.AddInstructions(instructions)

	return nil
}

func (gen *Generator) GenerateCond(args []Sexp) error {
	if len(args) % 2 == 0 {
		return errors.New("missing default case")
	}

	subgen := NewGenerator(gen.env)
	err := subgen.Generate(args[len(args) - 1])
	if err != nil {
		return err
	}
	instructions := subgen.instructions

	for i := len(args) / 2 - 1; i >= 0; i-- {
		subgen.Reset()
		err := subgen.Generate(args[2 * i])
		if err != nil {
			return err
		}
		pred_code := subgen.instructions

		subgen.Reset()
		err = subgen.Generate(args[2 * i + 1])
		if err != nil {
			return err
		}
		body_code := subgen.instructions

		subgen.Reset()
		subgen.AddInstructions(pred_code)
		subgen.AddInstruction(BranchInstr{false, len(body_code) + 2})
		subgen.AddInstructions(body_code)
		subgen.AddInstruction(JumpInstr{len(instructions) + 1})
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

func (gen *Generator) GenerateCallBySymbol(sym SexpSymbol, args []Sexp) error {
	switch sym.name {
	case "and":
		return gen.GenerateShortCircuit(false, args)
	case "or":
		return gen.GenerateShortCircuit(true, args)
	case "cond":
		return gen.GenerateCond(args)
	case "quote":
		return gen.GenerateQuote(args)
	case "def":
		return gen.GenerateDef(args)
	case "fn":
		return gen.GenerateFn(args)
	case "defn":
		return gen.GenerateDefn(args)
	}
	gen.GenerateAll(args)
	gen.AddInstruction(CallInstr{sym, len(args)})
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
		return gen.GenerateCallBySymbol(head, arr)
	}
	return gen.GenerateDispatch(expr.head, arr)
}

func (gen *Generator) Generate(expr Sexp) error {
	switch e := expr.(type) {
	case SexpSymbol:
		gen.AddInstruction(GetInstr{e})
		return nil
	case SexpPair:
		if IsList(e) {
			return gen.GenerateCall(e)
		} else {
			gen.AddInstruction(PushInstr{expr})
		}
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
}
