package zygo

import (
	"fmt"
	tm "github.com/glycerine/tmframe"
	"reflect"
	"time"
)

// The Go Type Registry
// ====================
//
// simply decide upon a name, and add a maker
// function for that returns a pointer to your struct.
// The simply add to the init() function below.
//
// The env parameter to your MakeGoStructFunc()
// function is there is case you want to initialize
// your struct differently depending on the content
// of its context, but this is not commonly needed.
// Also, the factory method *must* support the
// env parameter being nil and still return a
// sensible, usable value. The factory will be called
// with env = nil during init() time.
//
// The repl will automatically do a (defmap record)
// for each record defined in the registry. e.g.
// for snoopy, hornet, hellcat, etc.
//
var GoStructRegistry GoStructRegistryType

// the registry type
type GoStructRegistryType struct {
	// comprehensive
	Registry map[string]*RegisteredType

	// only init-time builtins
	Builtin map[string]*RegisteredType

	// later, user-defined types
	Userdef map[string]*RegisteredType
}

// consistently ordered list of all registered types (created at init time).
var ListRegisteredTypes = []string{}

func (r *GoStructRegistryType) RegisterBuiltin(name string, e *RegisteredType) {
	r.register(name, e, false)
	e.IsUser = false
}

func (r *GoStructRegistryType) register(name string, e *RegisteredType, isUser bool) {
	if !e.initDone {
		e.Init()
	}
	e.RegisteredName = name
	e.Aliases[name] = true
	e.Aliases[e.ReflectName] = true

	_, found := r.Registry[name]
	if !found {
		ListRegisteredTypes = append(ListRegisteredTypes, name)
	}
	_, found2 := r.Registry[e.ReflectName]
	if !found2 {
		ListRegisteredTypes = append(ListRegisteredTypes, e.ReflectName)
	}

	if isUser {
		r.Userdef[name] = e
	} else {
		r.Builtin[name] = e
	}
	r.Registry[name] = e
	r.Registry[e.ReflectName] = e
}

func (e *RegisteredType) Init() {
	e.Aliases = make(map[string]bool)
	val, err := e.Factory(nil)
	panicOn(err)
	if val != nil {
		e.ValueCache = reflect.ValueOf(val)
		e.TypeCache = e.ValueCache.Type()
		e.PointerName = fmt.Sprintf("%T", val)
		e.ReflectName = e.PointerName[1:]
		e.DisplayAs = e.ReflectName
	}
	e.initDone = true
}

func (r *GoStructRegistryType) RegisterUserdef(name string, e *RegisteredType, hasShadowStruct bool) {
	r.register(name, e, true)
	e.IsUser = true
	e.hasShadowStruct = hasShadowStruct
}

func (r *GoStructRegistryType) Lookup(name string) *RegisteredType {
	return r.Registry[name]
}

// the type of all maker functions
type MakeGoStructFunc func(env *Glisp) (interface{}, error)

type RegisteredType struct {
	initDone        bool
	hasShadowStruct bool

	Constructor    *SexpFunction
	RegisteredName string
	Factory        MakeGoStructFunc
	GenDefMap      bool
	ValueCache     reflect.Value
	TypeCache      reflect.Type
	PointerName    string
	ReflectName    string
	IsUser         bool
	Aliases        map[string]bool
	DisplayAs      string
}

func (p *RegisteredType) SexpString() string {
	return p.DisplayAs
}

func NewRegisteredType(f MakeGoStructFunc) *RegisteredType {
	rt := &RegisteredType{Factory: f}
	rt.Init()
	return rt
}

// builtin known Go Structs
// NB these are used to test the functionality of the
//    Go integration.
//
func init() {
	GoStructRegistry = GoStructRegistryType{
		Registry: make(map[string]*RegisteredType),
		Builtin:  make(map[string]*RegisteredType),
		Userdef:  make(map[string]*RegisteredType),
	}

	gsr := &GoStructRegistry

	// add go builtin types
	// ====================

	byteEntry := &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(byte), nil
	}}
	gsr.RegisterBuiltin("byte", byteEntry)
	gsr.RegisterBuiltin("uint8", byteEntry)

	gsr.RegisterBuiltin("int", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(int), nil
	}})
	gsr.RegisterBuiltin("uint16", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(uint16), nil
	}})
	gsr.RegisterBuiltin("uint32", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(uint32), nil
	}})
	gsr.RegisterBuiltin("uint64", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(uint64), nil
	}})
	gsr.RegisterBuiltin("int8", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(int8), nil
	}})
	gsr.RegisterBuiltin("int16", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(int16), nil
	}})
	gsr.RegisterBuiltin("int32", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(int32), nil
	}})
	gsr.RegisterBuiltin("int64", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(int64), nil
	}})
	gsr.RegisterBuiltin("float32", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(float32), nil
	}})

	gsr.RegisterBuiltin("float64", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(float64), nil
	}})

	gsr.RegisterBuiltin("complex64", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(complex64), nil
	}})

	gsr.RegisterBuiltin("complex128", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(complex128), nil
	}})

	gsr.RegisterBuiltin("bool", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(bool), nil
	}})

	gsr.RegisterBuiltin("string", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(string), nil
	}})

	gsr.RegisterBuiltin("time.Time", &RegisteredType{GenDefMap: false, Factory: func(env *Glisp) (interface{}, error) {
		return new(time.Time), nil
	}})

	gsr.RegisterUserdef("tm.Frame", &RegisteredType{GenDefMap: true, Factory: func(env *Glisp) (interface{}, error) {
		return new(tm.Frame), nil
	}}, true)

	// demo and user defined structs
	gsr.RegisterUserdef("event-demo", &RegisteredType{GenDefMap: true, Factory: func(env *Glisp) (interface{}, error) {
		return &Event{}, nil
	}}, true)
	gsr.RegisterUserdef("person-demo", &RegisteredType{GenDefMap: true, Factory: func(env *Glisp) (interface{}, error) {
		return &Person{}, nil
	}}, true)
	gsr.RegisterUserdef("snoopy", &RegisteredType{GenDefMap: true, Factory: func(env *Glisp) (interface{}, error) {
		return &Snoopy{}, nil
	}}, true)
	gsr.RegisterUserdef("hornet", &RegisteredType{GenDefMap: true, Factory: func(env *Glisp) (interface{}, error) {
		return &Hornet{}, nil
	}}, true)
	gsr.RegisterUserdef("hellcat", &RegisteredType{GenDefMap: true, Factory: func(env *Glisp) (interface{}, error) {
		return &Hellcat{}, nil
	}}, true)
	gsr.RegisterUserdef("weather", &RegisteredType{GenDefMap: true, Factory: func(env *Glisp) (interface{}, error) {
		return &Weather{}, nil
	}}, true)

}

func TypeListFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	narg := len(args)
	if narg != 0 {
		return SexpNull, WrongNargs
	}
	r := ListRegisteredTypes
	s := make([]Sexp, len(r))
	for i := range r {
		s[i] = SexpStr{S: r[i]}
	}
	return SexpArray(s), nil
}
