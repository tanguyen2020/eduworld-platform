// Package expr is the safe evaluator of parameterized questions (F023 US4, QUESTION_BANK §5, FR-QB-008): numbers,
// declared variables, + − × ÷ ^, comparisons, and/or/not and parentheses — nothing else. There are no function calls,
// no member access and no other identifiers; the AST depth, the expression length and the exponent are bounded, and
// arithmetic is exact (math/big.Rat). Pure Go, no I/O.
package expr

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits of an expression.
const (
	MaxLength   = 255
	MaxDepth    = 32
	MaxExponent = 10
	maxDigits   = 400 // numerator/denominator size bound (bits/3.3), guards overflow by repeated powers
)

// Errors of parsing and evaluation.
var (
	ErrSyntax      = errors.New("expr: syntax error")
	ErrIdentifier  = errors.New("expr: unknown identifier")
	ErrTooLong     = errors.New("expr: expression too long")
	ErrTooDeep     = errors.New("expr: expression nested too deeply")
	ErrDivZero     = errors.New("expr: division by zero")
	ErrExponent    = errors.New("expr: exponent must be an integer between -10 and 10")
	ErrType        = errors.New("expr: type mismatch")
	ErrOutOfBounds = errors.New("expr: number too large")
)

// Value is a number or a boolean.
type Value struct {
	Num  *big.Rat
	Bool bool
	IsB  bool
}

// Node is a parsed expression.
type Node interface {
	eval(vars map[string]*big.Rat) (Value, error)
}

type num struct{ v *big.Rat }
type ident struct{ name string }
type unary struct {
	op string
	x  Node
}
type binary struct {
	op   string
	l, r Node
}

type token struct {
	kind string // num, id, op, (, ), eof
	text string
}

func lex(s string) ([]token, error) {
	var out []token
	rs := []rune(s)
	for i := 0; i < len(rs); {
		c := rs[i]
		switch {
		case unicode.IsSpace(c):
			i++
		case unicode.IsDigit(c) || (c == '.' && i+1 < len(rs) && unicode.IsDigit(rs[i+1])):
			j := i
			for j < len(rs) && (unicode.IsDigit(rs[j]) || rs[j] == '.') {
				j++
			}
			out = append(out, token{"num", string(rs[i:j])})
			i = j
		case unicode.IsLetter(c) || c == '_':
			j := i
			for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j]) || rs[j] == '_') {
				j++
			}
			w := string(rs[i:j])
			switch w {
			case "and":
				out = append(out, token{"op", "&&"})
			case "or":
				out = append(out, token{"op", "||"})
			case "not":
				out = append(out, token{"op", "!"})
			default:
				out = append(out, token{"id", w})
			}
			i = j
		case c == '(' || c == ')':
			out = append(out, token{string(c), string(c)})
			i++
		default:
			two := ""
			if i+1 < len(rs) {
				two = string(rs[i : i+2])
			}
			switch two {
			case "==", "!=", "<=", ">=", "&&", "||":
				out = append(out, token{"op", two})
				i += 2
				continue
			}
			switch c {
			case '+', '-', '*', '/', '^', '<', '>', '!':
				out = append(out, token{"op", string(c)})
			case '×':
				out = append(out, token{"op", "*"})
			case '÷', ':':
				out = append(out, token{"op", "/"})
			case '−':
				out = append(out, token{"op", "-"})
			case '=':
				out = append(out, token{"op", "=="})
			default:
				return nil, fmt.Errorf("%w: unexpected %q", ErrSyntax, c)
			}
			i++
		}
	}
	return append(out, token{"eof", ""}), nil
}

var binPrec = map[string]int{"||": 1, "&&": 2, "==": 3, "!=": 3, "<": 4, "<=": 4, ">": 4, ">=": 4, "+": 5, "-": 5, "*": 6, "/": 6, "^": 8}

type parser struct {
	toks  []token
	pos   int
	vars  map[string]bool
	depth int
}

func (p *parser) peek() token { return p.toks[p.pos] }
func (p *parser) next() token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

// Parse parses expression s whose identifiers must be in vars (F023-AC-15): anything else, a call (identifier
// followed by a parenthesis) or a syntax error is refused.
func Parse(s string, vars map[string]bool) (Node, error) {
	if utf8.RuneCountInString(s) > MaxLength {
		return nil, ErrTooLong
	}
	toks, err := lex(s)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, vars: vars}
	n, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	if p.peek().kind != "eof" {
		return nil, fmt.Errorf("%w: unexpected %q", ErrSyntax, p.peek().text)
	}
	return n, nil
}

func (p *parser) expr(minPrec int) (Node, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > MaxDepth {
		return nil, ErrTooDeep
	}
	left, err := p.unary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		prec, ok := binPrec[t.text]
		if t.kind != "op" || !ok || prec < minPrec {
			return left, nil
		}
		p.next()
		next := prec + 1
		if t.text == "^" {
			next = prec // right associative
		}
		right, err := p.expr(next)
		if err != nil {
			return nil, err
		}
		left = binary{op: t.text, l: left, r: right}
	}
}

func (p *parser) unary() (Node, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > MaxDepth {
		return nil, ErrTooDeep
	}
	t := p.next()
	switch t.kind {
	case "num":
		r, ok := new(big.Rat).SetString(t.text)
		if !ok || strings.Count(t.text, ".") > 1 {
			return nil, fmt.Errorf("%w: bad number %q", ErrSyntax, t.text)
		}
		return num{r}, nil
	case "id":
		if p.peek().kind == "(" {
			return nil, fmt.Errorf("%w: function calls are not allowed (%s)", ErrIdentifier, t.text)
		}
		if !p.vars[t.text] {
			return nil, fmt.Errorf("%w: %s", ErrIdentifier, t.text)
		}
		return ident{t.text}, nil
	case "(":
		n, err := p.expr(0)
		if err != nil {
			return nil, err
		}
		if p.next().kind != ")" {
			return nil, fmt.Errorf("%w: missing )", ErrSyntax)
		}
		return n, nil
	case "op":
		if t.text == "-" || t.text == "+" || t.text == "!" {
			x, err := p.expr(7)
			if err != nil {
				return nil, err
			}
			return unary{op: t.text, x: x}, nil
		}
	}
	return nil, fmt.Errorf("%w: unexpected %q", ErrSyntax, t.text)
}

// Eval evaluates n with the variable values.
func Eval(n Node, vars map[string]*big.Rat) (Value, error) { return n.eval(vars) }

// Number evaluates a numeric expression.
func Number(n Node, vars map[string]*big.Rat) (*big.Rat, error) {
	v, err := n.eval(vars)
	if err != nil {
		return nil, err
	}
	if v.IsB {
		return nil, ErrType
	}
	return v.Num, nil
}

// Truth evaluates a boolean expression (constraints).
func Truth(n Node, vars map[string]*big.Rat) (bool, error) {
	v, err := n.eval(vars)
	if err != nil {
		return false, err
	}
	if !v.IsB {
		return false, ErrType
	}
	return v.Bool, nil
}

func (n num) eval(map[string]*big.Rat) (Value, error) { return Value{Num: new(big.Rat).Set(n.v)}, nil }

func (n ident) eval(vars map[string]*big.Rat) (Value, error) {
	v, ok := vars[n.name]
	if !ok || v == nil {
		return Value{}, fmt.Errorf("%w: %s", ErrIdentifier, n.name)
	}
	return Value{Num: new(big.Rat).Set(v)}, nil
}

func (n unary) eval(vars map[string]*big.Rat) (Value, error) {
	x, err := n.x.eval(vars)
	if err != nil {
		return Value{}, err
	}
	switch n.op {
	case "!":
		if !x.IsB {
			return Value{}, ErrType
		}
		return Value{IsB: true, Bool: !x.Bool}, nil
	case "-":
		if x.IsB {
			return Value{}, ErrType
		}
		return Value{Num: new(big.Rat).Neg(x.Num)}, nil
	}
	if x.IsB {
		return Value{}, ErrType
	}
	return x, nil
}

func bounded(r *big.Rat) (Value, error) {
	if r.Num().BitLen() > maxDigits*10/3 || r.Denom().BitLen() > maxDigits*10/3 {
		return Value{}, ErrOutOfBounds
	}
	return Value{Num: r}, nil
}

func (n binary) eval(vars map[string]*big.Rat) (Value, error) {
	l, err := n.l.eval(vars)
	if err != nil {
		return Value{}, err
	}
	if n.op == "&&" || n.op == "||" {
		if !l.IsB {
			return Value{}, ErrType
		}
		if (n.op == "&&" && !l.Bool) || (n.op == "||" && l.Bool) {
			return l, nil
		}
		r, err := n.r.eval(vars)
		if err != nil {
			return Value{}, err
		}
		if !r.IsB {
			return Value{}, ErrType
		}
		return r, nil
	}
	r, err := n.r.eval(vars)
	if err != nil {
		return Value{}, err
	}
	if l.IsB || r.IsB {
		if (n.op == "==" || n.op == "!=") && l.IsB && r.IsB {
			return Value{IsB: true, Bool: (l.Bool == r.Bool) == (n.op == "==")}, nil
		}
		return Value{}, ErrType
	}
	a, b := l.Num, r.Num
	switch n.op {
	case "+":
		return bounded(new(big.Rat).Add(a, b))
	case "-":
		return bounded(new(big.Rat).Sub(a, b))
	case "*":
		return bounded(new(big.Rat).Mul(a, b))
	case "/":
		if b.Sign() == 0 {
			return Value{}, ErrDivZero
		}
		return bounded(new(big.Rat).Quo(a, b))
	case "^":
		if !b.IsInt() || b.Num().CmpAbs(big.NewInt(MaxExponent)) > 0 {
			return Value{}, ErrExponent
		}
		e := int(b.Num().Int64())
		out := big.NewRat(1, 1)
		for range abs(e) {
			out.Mul(out, a)
		}
		if e < 0 {
			if out.Sign() == 0 {
				return Value{}, ErrDivZero
			}
			out.Inv(out)
		}
		return bounded(out)
	}
	c := a.Cmp(b)
	var ok bool
	switch n.op {
	case "==":
		ok = c == 0
	case "!=":
		ok = c != 0
	case "<":
		ok = c < 0
	case "<=":
		ok = c <= 0
	case ">":
		ok = c > 0
	case ">=":
		ok = c >= 0
	default:
		return Value{}, ErrSyntax
	}
	return Value{IsB: true, Bool: ok}, nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
