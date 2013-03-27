// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of conversions.

package types

import (
	"go/ast"
)

// conversion typechecks the type conversion conv to type typ. iota is the current
// value of iota or -1 if iota doesn't have a value in the current context. The result
// of the conversion is returned via x. If the conversion has type errors, the returned
// x is marked as invalid (x.mode == invalid).
//
func (check *checker) conversion(x *operand, conv *ast.CallExpr, typ Type, iota int) {
	// all conversions have one argument
	if len(conv.Args) != 1 {
		check.invalidOp(conv.Pos(), "%s conversion requires exactly one argument", conv)
		goto Error
	}

	// evaluate argument
	check.expr(x, conv.Args[0], nil, iota)
	if x.mode == invalid {
		goto Error
	}

	if x.mode == constant && isConstType(typ) {
		// constant conversion
		typ := underlying(typ).(*Basic)
		// For now just implement string(x) where x is an integer,
		// as a temporary work-around for issue 4982, which is a
		// common issue.
		if typ.Kind == String {
			switch {
			case x.isInteger():
				codepoint, ok := x.val.(int64)
				if !ok {
					// absolute value too large (or unknown) for conversion;
					// same as converting any other out-of-range value - let
					// string(codepoint) do the work
					codepoint = -1
				}
				x.val = string(codepoint)
			case isString(x.typ):
				// nothing to do
			default:
				goto ErrorMsg
			}
		}
		// TODO(gri) verify the remaining conversions.
	} else {
		// non-constant conversion
		if !x.isConvertible(check.ctxt, typ) {
			goto ErrorMsg
		}
		x.mode = value
	}

	// the conversion argument types are final; for now we just use x.typ
	// TODO(gri) What should the type used here be? The spec is unclear.
	//           See also disabled test cases in testdata/shifts.src, shifts8().
	check.updateExprType(x.expr, x.typ, true)

	check.conversions[conv] = true // for cap/len checking
	x.expr = conv
	x.typ = typ
	return

ErrorMsg:
	check.invalidOp(conv.Pos(), "cannot convert %s to %s", x, typ)
Error:
	x.mode = invalid
	x.expr = conv
}

func (x *operand) isConvertible(ctxt *Context, T Type) bool {
	// "x is assignable to T"
	if x.isAssignable(ctxt, T) {
		return true
	}

	// "x's type and T have identical underlying types"
	V := x.typ
	Vu := underlying(V)
	Tu := underlying(T)
	if IsIdentical(Vu, Tu) {
		return true
	}

	// "x's type and T are unnamed pointer types and their pointer base types have identical underlying types"
	if V, ok := V.(*Pointer); ok {
		if T, ok := T.(*Pointer); ok {
			if IsIdentical(underlying(V.Base), underlying(T.Base)) {
				return true
			}
		}
	}

	// "x's type and T are both integer or floating point types"
	if (isInteger(V) || isFloat(V)) && (isInteger(T) || isFloat(T)) {
		return true
	}

	// "x's type and T are both complex types"
	if isComplex(V) && isComplex(T) {
		return true
	}

	// "x is an integer or a slice of bytes or runes and T is a string type"
	if (isInteger(V) || isBytesOrRunes(Vu)) && isString(T) {
		return true
	}

	// "x is a string and T is a slice of bytes or runes"
	if isString(V) && isBytesOrRunes(Tu) {
		return true
	}

	// package unsafe:
	// "any pointer or value of underlying type uintptr can be converted into a unsafe.Pointer"
	if (isPointer(Vu) || isUintptr(Vu)) && isUnsafePointer(T) {
		return true
	}
	// "and vice versa"
	if isUnsafePointer(V) && (isPointer(Tu) || isUintptr(Tu)) {
		return true
	}

	return false
}

func isUintptr(typ Type) bool {
	t, ok := typ.(*Basic)
	return ok && t.Kind == Uintptr
}

func isUnsafePointer(typ Type) bool {
	t, ok := typ.(*Basic)
	return ok && t.Kind == UnsafePointer
}

func isPointer(typ Type) bool {
	_, ok := typ.(*Pointer)
	return ok
}

func isBytesOrRunes(typ Type) bool {
	if s, ok := typ.(*Slice); ok {
		t, ok := underlying(s.Elt).(*Basic)
		return ok && (t.Kind == Byte || t.Kind == Rune)
	}
	return false
}