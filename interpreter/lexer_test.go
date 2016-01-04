package glisp

import (
	"bytes"
	"testing"

	"github.com/shurcooL/go-goon"
	cv "github.com/smartystreets/goconvey/convey"
)

func Test001LexerPositionRecordingWorks(t *testing.T) {

	cv.Convey(`Given a function definition in a stream, the token positions should reflect the span of the source code function definition, so we can retreive the functions definition easily`, t, func() {

		str := `(defn hello [] "greetings!")`
		env := NewGlisp()
		stream := bytes.NewBuffer([]byte(str))
		lexer := NewLexerFromStream(stream)
		expressions, err := ParseTokens(env, lexer)
		panicOn(err)
		goon.Dump(expressions[0])
		cv.So(expressions[0].SexpString(), cv.ShouldEqual, `(defn hello [] "greetings!")`)
	})
}
