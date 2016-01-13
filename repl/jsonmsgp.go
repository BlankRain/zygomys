package zygo

import (
	"bytes"
	//"encoding/json"
	"fmt"
	"github.com/shurcooL/go-goon"
	"github.com/ugorji/go/codec"
	//"io"
	"reflect"
	"sort"
	"strings"
)

//go:generate msgp

type Event struct {
	Id     int      `json:"id" msg:"id"`
	User   Person   `json:"user" msg:"user"`
	Flight string   `json:"flight" msg:"flight"`
	Pilot  []string `json:"pilot" msg:"pilot"`
}

type Person struct {
	First string `json:"first" msg:"first"`
	Last  string `json:"last" msg:"last"`
}

func (ev *Event) DisplayEvent(from string) {
	fmt.Printf("%s %#v", from, ev)
}

/*
 Conversion map

 Go map[string]interface{}  <--(1)--> lisp
   ^                                  ^
   |                                 /
  (2)   ------------ (4) -----------/
   |   /
   V  V
 msgpack <--(3)--> go struct, strongly typed

(1) we provide these herein
     (a) SexpToGo()
     (b) GoToSexp()
(2) provided by ugorji/go/codec; see examples also herein
     (a) MsgpackToGo() / JsonToGo()
     (b) GoToMsgpack() / GoToJson()
(3) provided by tinylib/msgp, and by ugorji/go/codec
     by using pre-compiled or just decoding into an instance
     of the struct.
(4) see herein
     (a) SexpToMsgpack() and SexpToJson()
     (b) MsgpackToSexp(); uses (4) = (2) + (1)
*/
func JsonFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 1 {
		return SexpNull, WrongNargs
	}

	switch name {
	case "json":
		str := SexpToJson(args[0])
		return SexpRaw([]byte(str)), nil
	case "unjson":
		raw, isRaw := args[0].(SexpRaw)
		if !isRaw {
			return SexpNull, fmt.Errorf("unjson error: SexpRaw required, but we got %T instead.", args[0])
		}
		return JsonToSexp([]byte(raw), env)
	case "msgpack":
		by, _ := SexpToMsgpack(args[0])
		return SexpRaw([]byte(by)), nil
	case "unmsgpack":
		raw, isRaw := args[0].(SexpRaw)
		if !isRaw {
			return SexpNull, fmt.Errorf("unmsgpack error: SexpRaw required, but we got %T instead.", args[0])
		}
		return MsgpackToSexp([]byte(raw), env)
	default:
		return SexpNull, fmt.Errorf("JsonFunction error: unrecognized function name: '%s'", name)
	}
}

// json -> sexp. env is needed to handle symbols correctly
func JsonToSexp(json []byte, env *Glisp) (Sexp, error) {
	iface, err := JsonToGo(json)
	if err != nil {
		return nil, err
	}
	return GoToSexp(iface, env)
}

// sexp -> json
func SexpToJson(exp Sexp) string {
	switch e := exp.(type) {
	case SexpHash:
		return e.jsonHashHelper()
	case SexpArray:
		return e.jsonArrayHelper()
	case SexpSymbol:
		return `"` + e.name + `"`
	default:
		return exp.SexpString()
	}
}

func (hash *SexpHash) jsonHashHelper() string {
	str := fmt.Sprintf(`{"Atype":"%s", `, *hash.TypeName)

	ko := []string{}
	n := len(*hash.KeyOrder)
	if n == 0 {
		return str[:len(str)-2] + "}"
	}

	for _, key := range *hash.KeyOrder {
		keyst := key.SexpString()
		ko = append(ko, keyst)
		val, err := hash.HashGet(key)
		if err == nil {
			str += `"` + keyst + `":`
			str += string(SexpToJson(val)) + `, `
		} else {
			panic(err)
		}
	}

	str += `"zKeyOrder":[`
	for _, key := range ko {
		str += `"` + key + `", `
	}
	if n > 0 {
		str = str[:len(str)-2]
	}
	str += "]}"

	VPrintf("\n\n final ToJson() str = '%s'\n", str)
	return str
}

func (arr *SexpArray) jsonArrayHelper() string {
	if len(*arr) == 0 {
		return "[]"
	}

	str := "[" + SexpToJson((*arr)[0])
	for _, sexp := range (*arr)[1:] {
		str += ", " + SexpToJson(sexp)
	}
	return str + "]"
}

type msgpackHelper struct {
	initialized bool
	mh          codec.MsgpackHandle
	jh          codec.JsonHandle
}

func (m *msgpackHelper) init() {
	if m.initialized {
		return
	}

	m.mh.MapType = reflect.TypeOf(map[string]interface{}(nil))

	// configure extensions
	// e.g. for msgpack, define functions and enable Time support for tag 1
	//does this make a differenece? m.mh.AddExt(reflect.TypeOf(time.Time{}), 1, timeEncExt, timeDecExt)
	m.mh.RawToString = true
	m.mh.WriteExt = true
	m.mh.SignedInteger = true
	m.mh.Canonical = true // sort maps before writing them

	// JSON
	m.jh.MapType = reflect.TypeOf(map[string]interface{}(nil))
	m.jh.SignedInteger = true
	m.jh.Canonical = true // sort maps before writing them

	m.initialized = true
}

var msgpHelper msgpackHelper

func init() {
	msgpHelper.init()
}

// translate to sexp -> json -> go -> msgpack
// returns both the msgpack []bytes and the go intermediary
func SexpToMsgpack(exp Sexp) ([]byte, interface{}) {

	json := []byte(SexpToJson(exp))
	iface, err := JsonToGo(json)
	panicOn(err)
	by, err := GoToMsgpack(iface)
	panicOn(err)
	return by, iface
}

// json -> go
func JsonToGo(json []byte) (interface{}, error) {
	var iface interface{}

	decoder := codec.NewDecoderBytes(json, &msgpHelper.jh)
	err := decoder.Decode(&iface)
	if err != nil {
		panic(err)
	}
	//fmt.Printf("\n decoded type : %T\n", iface)
	//fmt.Printf("\n decoded value: %#v\n", iface)
	//goon.Dump(iface)
	return iface, nil
}

func GoToMsgpack(iface interface{}) ([]byte, error) {
	var w bytes.Buffer
	enc := codec.NewEncoder(&w, &msgpHelper.mh)
	err := enc.Encode(&iface)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

// go -> json
func GoToJson(iface interface{}) []byte {
	var w bytes.Buffer
	encoder := codec.NewEncoder(&w, &msgpHelper.jh)
	err := encoder.Encode(&iface)
	if err != nil {
		panic(err)
	}
	return w.Bytes()
}

// msgpack -> sexp
func MsgpackToSexp(msgp []byte, env *Glisp) (Sexp, error) {
	iface, err := MsgpackToGo(msgp)
	if err != nil {
		return nil, fmt.Errorf("MsgpackToSexp failed at MsgpackToGo step: '%s", err)
	}
	sexp, err := GoToSexp(iface, env)
	if err != nil {
		return nil, fmt.Errorf("MsgpackToSexp failed at GoToSexp step: '%s", err)
	}
	return sexp, nil
}

// msgpack -> go
func MsgpackToGo(msgp []byte) (interface{}, error) {

	var iface interface{}
	dec := codec.NewDecoderBytes(msgp, &msgpHelper.mh)
	err := dec.Decode(&iface)
	if err != nil {
		return nil, err
	}

	//fmt.Printf("\n decoded type : %T\n", iface)
	//fmt.Printf("\n decoded value: %#v\n", iface)
	return iface, nil
}

// convert iface, which will typically be map[string]interface{},
// into an s-expression
func GoToSexp(iface interface{}, env *Glisp) (Sexp, error) {
	return decodeGoToSexpHelper(iface, 0, env, false), nil
}

func decodeGoToSexpHelper(r interface{}, depth int, env *Glisp, preferSym bool) (s Sexp) {

	VPrintf("decodeHelper() at depth %d, decoded type is %T\n", depth, r)
	switch val := r.(type) {
	case string:
		VPrintf("depth %d found string case: val = %#v\n", depth, val)
		if preferSym {
			return env.MakeSymbol(val)
		}
		return SexpStr(val)

	case int:
		VPrintf("depth %d found int case: val = %#v\n", depth, val)
		return SexpInt(val)

	case int32:
		VPrintf("depth %d found int32 case: val = %#v\n", depth, val)
		return SexpInt(val)

	case int64:
		VPrintf("depth %d found int64 case: val = %#v\n", depth, val)
		return SexpInt(val)

	case float64:
		VPrintf("depth %d found float64 case: val = %#v\n", depth, val)
		return SexpFloat(val)

	case []interface{}:
		VPrintf("depth %d found []interface{} case: val = %#v\n", depth, val)

		slice := []Sexp{}
		for i := range val {
			slice = append(slice, decodeGoToSexpHelper(val[i], depth+1, env, preferSym))
		}
		return SexpArray(slice)

	case map[string]interface{}:

		VPrintf("depth %d found map[string]interface case: val = %#v\n", depth, val)
		sortedMapKey, sortedMapVal := makeSortedSlicesFromMap(val)

		pairs := make([]Sexp, 0)

		typeName := "hash"
		var keyOrd Sexp
		foundzKeyOrder := false
		for i := range sortedMapKey {
			// special field storing the name of our record (defmap) type.
			VPrintf("\n i=%d sortedMapVal type %T, value=%v\n", i, sortedMapVal[i], sortedMapVal[i])
			VPrintf("\n i=%d sortedMapKey type %T, value=%v\n", i, sortedMapKey[i], sortedMapKey[i])
			if sortedMapKey[i] == "zKeyOrder" {
				keyOrd = decodeGoToSexpHelper(sortedMapVal[i], depth+1, env, true)
				foundzKeyOrder = true
			} else if sortedMapKey[i] == "Atype" {
				tn, isString := sortedMapVal[i].(string)
				if isString {
					typeName = string(tn)
				}
			} else {
				sym := env.MakeSymbol(sortedMapKey[i])
				pairs = append(pairs, sym)
				ele := decodeGoToSexpHelper(sortedMapVal[i], depth+1, env, preferSym)
				pairs = append(pairs, ele)
			}
		}
		hash, err := MakeHash(pairs, typeName, env)
		if foundzKeyOrder {
			err = SetHashKeyOrder(&hash, keyOrd)
			panicOn(err)
		}
		panicOn(err)
		return hash

	case []byte:
		VPrintf("depth %d found []byte case: val = %#v\n", depth, val)

		return SexpRaw(val)

	case nil:
		return SexpNull

	case bool:
		return SexpBool(val)

	default:
		fmt.Printf("unknown type in type switch, val = %#v.  type = %T.\n", val, val)
	}

	return s
}

//msgp:ignore mapsorter KiSlice

type mapsorter struct {
	key   string
	iface interface{}
}

type KiSlice []*mapsorter

func (a KiSlice) Len() int           { return len(a) }
func (a KiSlice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a KiSlice) Less(i, j int) bool { return a[i].key < a[j].key }

func makeSortedSlicesFromMap(m map[string]interface{}) ([]string, []interface{}) {
	key := make([]string, len(m))
	val := make([]interface{}, len(m))
	so := make(KiSlice, 0)
	for k, i := range m {
		so = append(so, &mapsorter{key: k, iface: i})
	}
	sort.Sort(so)
	for i := range so {
		key[i] = so[i].key
		val[i] = so[i].iface
	}
	return key, val
}

// translate an Sexpr to a go value that doesn't
// depend on any Sexp/Glisp types. Glisp maps
// will get turned into map[string]interface{}.
// This is mostly just an exercise in type conversion.
func SexpToGo(sexp Sexp, env *Glisp) interface{} {

	switch e := sexp.(type) {
	case SexpRaw:
		return []byte(e)
	case SexpArray:
		ar := make([]interface{}, len(e))
		for i, ele := range e {
			ar[i] = SexpToGo(ele, env)
		}
		return ar
	case SexpInt:
		// ugorji msgpack will give us int64 not int,
		// so match that to make the decodings comparable.
		return int64(e)
	case SexpStr:
		return string(e)
	case SexpChar:
		return rune(e)
	case SexpFloat:
		return float64(e)
	case SexpHash:
		m := make(map[string]interface{})
		for _, arr := range e.Map {
			for _, pair := range arr {
				key := SexpToGo(pair.head, env)
				val := SexpToGo(pair.tail, env)
				keyString, isStringKey := key.(string)
				if !isStringKey {
					panic(fmt.Errorf("key '%v' should have been a string, but was not.", key))
				}
				m[keyString] = val
			}
		}
		m["Atype"] = *e.TypeName
		ko := make([]interface{}, 0)
		for _, k := range *e.KeyOrder {
			ko = append(ko, SexpToGo(k, env))
		}
		m["zKeyOrder"] = ko
		return m
	case SexpPair:
		// no conversion
		return e
	case SexpSymbol:
		return e.name
	case SexpFunction:
		// no conversion done
		return e
	case SexpSentinel:
		// no conversion done
		return e
	default:
		fmt.Printf("\n error: unknown type: %T in '%#v'\n", e, e)
	}
	return nil
}

func ToGoFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 1 {
		return SexpNull, WrongNargs
	}
	switch asHash := args[0].(type) {
	default:
		return SexpNull, fmt.Errorf("value must be a hash or defmap")
	case SexpHash:
		tn := *(asHash.TypeName)
		factory, hasMaker := GostructRegistry[tn]
		if !hasMaker {
			return SexpNull, fmt.Errorf("type '%s' not registered in GostructRegistry", tn)
		}
		newStruct := factory(env)
		_, err := SexpToGoStructs(asHash, newStruct, env)
		if err != nil {
			return SexpNull, err
		}
		asHash.GoShadowStruct = newStruct
		return SexpStr(fmt.Sprintf("%#v", newStruct)), nil
	}
	return SexpNull, nil
}

func GoonDumpFunction(env *Glisp, name string, args []Sexp) (Sexp, error) {
	if len(args) != 1 {
		return SexpNull, WrongNargs
	}
	fmt.Printf("\n")
	goon.Dump(args[0])
	return SexpNull, nil
}

// try to convert to registered go structs if possible,
// filling in the structure of target (should be a pointer).
func SexpToGoStructs(sexp Sexp, target interface{}, env *Glisp) (interface{}, error) {
	VPrintf("\n 88888 entering SexpToGoStructs() with sexp=%v and target=%#v of type %s\n", sexp, target, reflect.ValueOf(target).Type())
	defer func() {
		VPrintf("\n 99999 leaving SexpToGoStructs() with sexp=%v and target=%#v\n", sexp, target)
	}()

	if !IsExactlySinglePointer(target) {
		panic(fmt.Errorf("SexpToGoStructs() got bad target: was not *T single level pointer, but rather %s", reflect.ValueOf(target).Type()))
	}

	// target is a pointer to our payload.
	// targVa is a pointer to that same payload.
	targVa := reflect.ValueOf(target)
	targTyp := targVa.Type()
	targKind := targVa.Kind()
	targElemTyp := targTyp.Elem()
	targElemKind := targElemTyp.Kind()

	VPrintf("\n targVa is '%#v'\n", targVa)

	if targKind != reflect.Ptr {
		//		panic(fmt.Errorf("SexpToGoStructs got non-pointer type! was type %T/val=%#v.  targKind=%#v targTyp=%#v targVa=%#v", target, target, targKind, targTyp, targVa))
	}

	switch src := sexp.(type) {
	case SexpRaw:
		panic("unimplemented") //return []byte(src)
	case SexpArray:
		VPrintf("\n\n starting 5555555555 on SexpArray\n")
		if targElemKind != reflect.Array && targElemKind != reflect.Slice {
			panic(fmt.Errorf("tried to translate from SexpArray into non-array/type: %v", targKind))
		}
		// allocate the slice
		n := len(src)
		slc := reflect.MakeSlice(targElemTyp, 0, n)
		VPrintf("\n slc starts out as %v/type = %T\n", slc, slc.Interface())
		// if targ is *[]int, then targElem is []int, targElem.Elem() is int.
		eTyp := targElemTyp.Elem()
		for i, ele := range src {
			goElem := reflect.New(eTyp) // returns pointer to new value
			VPrintf("\n goElem = %#v before filling i=%d\n", goElem, i)
			if _, err := SexpToGoStructs(ele, goElem.Interface(), env); err != nil {
				return nil, err
			}
			VPrintf("\n goElem = %#v after filling i=%d\n", goElem, i)
			VPrintf("\n goElem.Elem() = %#v after filling i=%d\n", goElem.Elem(), i)
			slc = reflect.Append(slc, goElem.Elem())
			VPrintf("\n slc after i=%d is now %v\n", i, slc)
		}
		targVa.Elem().Set(slc)
		VPrintf("\n targVa is now %v\n", targVa)

	case SexpInt:
		// ugorji msgpack will give us int64 not int,
		// so match that to make the decodings comparable.
		targVa.Elem().SetInt(int64(src))
	case SexpStr:
		targVa.Elem().SetString(string(src))
	case SexpChar:
		panic("unimplemented") //return rune(src)
	case SexpFloat:
		targVa.Elem().SetFloat(float64(src))
	case SexpHash:
		VPrintf("\n ==== found SexpHash\n\n")
		tn := *(src.TypeName)
		if tn == "hash" {
			panic("not done here yet")
			// TODO: don't try to translate into a Go struct,
			// but instead... what? just a map[string]interface{}
			return nil, nil
		}

		switch targTyp.Elem().Kind() {
		case reflect.Interface:
			// could be an Interface like Flyer here, that contains the struct.
		case reflect.Struct:
			// typical case
		default:
			panic(fmt.Errorf("tried to translate from SexpHash record into non-struct/type: %v  / targType.Elem().Kind()=%v", targKind, targTyp.Elem().Kind()))
		}

		// use targVa, but check against the type in the registry for sanity/type checking.
		factory, hasMaker := GostructRegistry[tn]
		if !hasMaker {
			panic(fmt.Errorf("type '%s' not registered in GostructRegistry", tn))
			return nil, fmt.Errorf("type '%s' not registered in GostructRegistry", tn)
		}
		VPrintf("factory = %#v  targTyp.Kind=%s\n", factory, targTyp.Kind())
		checkPtrStruct := factory(env)
		factOutputVal := reflect.ValueOf(checkPtrStruct)
		factType := factOutputVal.Type()
		if targTyp.Kind() == reflect.Ptr && targTyp.Elem().Kind() == reflect.Interface && factType.Implements(targTyp.Elem()) {
			VPrintf("\n accepting type check: %v implements %v\n", factType, targTyp)

			// also here we need to allocate an actual struct in place of
			// the interface

			// caller has a pointer to an interface
			// and we just want to set that interface to point to us.
			targVa.Elem().Set(factOutputVal) // tell our caller

			// now fill into this concrete type
			targVa = factOutputVal // tell the code below
			targTyp = targVa.Type()
			targKind = targVa.Kind()

		} else if factType != targTyp {
			panic(fmt.Errorf("type checking failed compare the factor associated with SexpHash and the provided target *T: expected '%s' (associated with typename '%s' in the GostructRegistry) but saw '%s' type in target", tn, factType, targTyp))
		}
		//maploop:
		for _, arr := range src.Map {
			for _, pair := range arr {
				var recordKey string
				switch k := pair.head.(type) {
				case SexpStr:
					recordKey = string(k)
				case SexpSymbol:
					recordKey = k.name
				default:
					fmt.Printf("\n skipping field '%#v' which we don't know how to lookup.\n", pair.head)
					continue
				}
				// We've got to match pair.head to
				// one of the struct fields: we'll use
				// the json tags for that. Or their
				// full exact name if they didn't have
				// a json tag.
				VPrintf("\n JsonTagMap = %#v\n", (*src.JsonTagMap))
				det, found := (*src.JsonTagMap)[recordKey]
				if !found {
					// try once more, with uppercased version
					// of record key
					upperKey := strings.ToUpper(recordKey[:1]) + recordKey[1:]
					det, found = (*src.JsonTagMap)[upperKey]
					if !found {
						fmt.Printf("\n skipping field '%s' in this hash/which we could not find in the JsonTagMap\n", recordKey)
						continue
					}
				}
				VPrintf("\n\n ****  recordKey = '%s'\n\n", recordKey)
				VPrintf("\n we found in pair.tail: %T !\n", pair.tail)

				dref := targVa.Elem()
				VPrintf("\n deref = %#v / type %T\n", dref, dref)

				VPrintf("\n det = %#v\n", det)

				// fld should hold our target when
				// done recursing through any embedded structs.
				// TODO: handle embedded pointers to structs too.
				var fld reflect.Value
				VPrintf("\n we have an det.EmbedPath of '%#v'\n", det.EmbedPath)
				// drill down to the actual target
				fld = dref
				for i, p := range det.EmbedPath {
					VPrintf("about to call fld.Field(%d) on fld = '%#v'/type=%T\n", p.ChildFieldNum, fld, fld)
					fld = fld.Field(p.ChildFieldNum)
					VPrintf("\n dropping down i=%d through EmbedPath at '%s', fld = %#v \n", i, p.ChildName, fld)
				}
				VPrintf("\n fld = %#v \n", fld)

				// INVAR: fld points at our target to fill
				ptrFld := fld.Addr()
				VPrintf("\n ptrFld = %#v \n", ptrFld)

				_, err := SexpToGoStructs(pair.tail, ptrFld.Interface(), env)
				if err != nil {
					panic(err)
					return nil, err
				}
			}
		}
	case SexpPair:
		panic("unimplemented")
		// no conversion
		//return src
	case SexpSymbol:
		targVa.Elem().SetString(src.name)
	case SexpFunction:
		panic("unimplemented")
		// no conversion done
		//return src
	case SexpSentinel:
		panic("unimplemented")
		// no conversion done
		//return src
	default:
		fmt.Printf("\n error: unknown type: %T in '%#v'\n", src, src)
	}
	return target, nil
}

// A small set of important little buildling blocks.
// These demonstrate how to use reflect.
/*
(1) Tutorial on setting structs with reflect.Set()

http://play.golang.org/p/sDmFgZmGvv

package main

import (
"fmt"
"reflect"

)

type A struct {
  S string
}

func MakeA() interface{} {
  return &A{}
}

func main() {
   a1 := MakeA()
   a2 := MakeA()
   a2.(*A).S = "two"

   // now assign a2 -> a1 using reflect.
    targVa := reflect.ValueOf(&a1).Elem()
    targVa.Set(reflect.ValueOf(a2))
    fmt.Printf("a1 = '%#v' / '%#v'\n", a1, targVa.Interface())
}
// output
// a1 = '&main.A{S:"two"}' / '&main.A{S:"two"}'


(2) Tutorial on setting fields inside a struct with reflect.Set()

http://play.golang.org/p/1k4iQKVwUD

package main

import (
    "fmt"
    "reflect"
)

type A struct {
    S string
}

func main() {
    a1 := &A{}

    three := "three"

    fld := reflect.ValueOf(&a1).Elem().Elem().FieldByName("S")

    fmt.Printf("fld = %#v\n of type %T\n", fld, fld)
    fmt.Println("settability of fld:", fld.CanSet()) // true

    // now assign to field a1.S the string "three" using reflect.

    fld.Set(reflect.ValueOf(three))

    fmt.Printf("after fld.Set(): a1 = '%#v' \n", a1)
}

// output:
fld = ""
 of type reflect.Value
settability of fld: true
after fld.Set(): a1 = '&main.A{S:"three"}'

(3) Setting struct after passing through an function call interface{} param:

package main

import (
	"fmt"
	"reflect"
)

type A struct {
	S string
}

func main() {
	a1 := &A{}
	f(&a1)
	fmt.Printf("a1 = '%#v'\n", a1)
	// a1 = '&main.A{S:"two"}' / '&main.A{S:"two"}'
}

func f(i interface{}) {
	a2 := MakeA()
	a2.(*A).S = "two"

	// now assign a2 -> a1 using reflect.
	//targVa := reflect.ValueOf(&a1).Elem()
	targVa := reflect.ValueOf(i).Elem()
	targVa.Set(reflect.ValueOf(a2))
}

(4) using a function to do the Set(), and checking
    the received interface for correct type.
    Also: Using a function to set just one sub-field.

package main

import (
	"fmt"
	"reflect"
)

type A struct {
	S string
	R string
}

func main() {
	a1 := &A{}
	overwrite_contents_of_struct(a1)
	fmt.Printf("a1 = '%#v'\n", a1)

	// output:
	// yes, is single level pointer
	// a1 = '&main.A{S:"two", R:""}'

	assignToOnlyFieldR(a1)
	fmt.Printf("after assignToOnlyFieldR(a1):  a1 = '%#v'\n", a1)

	// output:
//	yes, is single level pointer
//	a1 = '&main.A{S:"two", R:""}'
//	yes, is single level pointer
//	fld = ""
//	 of type reflect.Value
//	settability of fld: true
//	after assignToOnlyFieldR(a1):  a1 = '&main.A{S:"two", R:"R has been altered"}'

}

func assignToOnlyFieldR(i interface{}) {
	if !IsExactlySinglePointer(i) {
		panic("not single level pointer")
	}
	fmt.Printf("yes, is single level pointer\n")

	altered := "R has been altered"

	fld := reflect.ValueOf(i).Elem().FieldByName("R")

	fmt.Printf("fld = %#v\n of type %T\n", fld, fld)
	fmt.Println("settability of fld:", fld.CanSet()) // true

	// now assign to field a1.S
	fld.Set(reflect.ValueOf(altered))
}

func overwrite_contents_of_struct(i interface{}) {
	// we want i to contain an *A, or a pointer-to struct.
	// So we can reassign *ptr = A' for a different content A'.

	if !IsExactlySinglePointer(i) {
		panic("not single level pointer")
	}
	fmt.Printf("yes, is single level pointer\n")

	a2 := &A{S: "two"}

	// now assign a2 -> a1 using reflect.
	targVa := reflect.ValueOf(i).Elem()
	targVa.Set(reflect.ValueOf(a2).Elem())
}

func IsExactlySinglePointer(target interface{}) bool {

	typ := reflect.ValueOf(target).Type()
	kind := typ.Kind()
	if kind != reflect.Ptr {
		return false
	}
	typ2 := typ.Elem()
	kind2 := typ2.Kind()
	if kind2 == reflect.Ptr {
		return false // two level pointer
	}
	return true
}

*/
