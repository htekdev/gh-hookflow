package expression

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/htekdev/gh-hookflow/internal/session"
)

// Context holds the evaluation context for expressions
type Context struct {
	Event            map[string]interface{}
	Env              map[string]string
	Steps            map[string]StepContext
	Functions        map[string]Function
	ContextFunctions map[string]ContextFunction
	SessionDir       string // Path to the session directory for transcript access
}

// StepContext holds the output of a previous step
type StepContext struct {
	Outputs map[string]string
	Outcome string // success, failure, cancelled, skipped
}

// Function represents a built-in function
type Function func(args ...interface{}) (interface{}, error)

// ContextFunction represents a built-in function that needs context access
type ContextFunction func(ctx *Context, args ...interface{}) (interface{}, error)

// NewContext creates a new evaluation context
func NewContext() *Context {
	ctx := &Context{
		Event:            make(map[string]interface{}),
		Env:              make(map[string]string),
		Steps:            make(map[string]StepContext),
		Functions:        make(map[string]Function),
		ContextFunctions: make(map[string]ContextFunction),
	}
	// Register built-in functions
	ctx.Functions["contains"] = builtinContains
	ctx.Functions["startsWith"] = builtinStartsWith
	ctx.Functions["endsWith"] = builtinEndsWith
	ctx.Functions["format"] = builtinFormat
	ctx.Functions["join"] = builtinJoin
	ctx.Functions["toJSON"] = builtinToJSON
	ctx.Functions["fromJSON"] = builtinFromJSON
	ctx.Functions["always"] = builtinAlways
	// Register context-aware functions
	ctx.ContextFunctions["success"] = builtinSuccess
	ctx.ContextFunctions["failure"] = builtinFailure
	ctx.ContextFunctions["cancelled"] = builtinCancelled
	ctx.ContextFunctions["transcript"] = builtinTranscript
	ctx.ContextFunctions["transcript_since"] = builtinTranscriptSince
	ctx.ContextFunctions["transcript_count"] = builtinTranscriptCount
	ctx.ContextFunctions["transcript_last"] = builtinTranscriptLast
	return ctx
}

// Evaluate evaluates an expression string against the context
func (ctx *Context) Evaluate(expr string) (interface{}, error) {
	// Parse the expression
	tokens, err := tokenize(expr)
	if err != nil {
		return nil, err
	}

	// Create evaluator
	eval := &evaluator{
		tokens: tokens,
		pos:    0,
		ctx:    ctx,
	}

	return eval.evaluate()
}

// EvaluateString evaluates an expression and returns a string result
func (ctx *Context) EvaluateString(input string) (string, error) {
	return ReplaceExpressions(input, func(expr string) (string, error) {
		result, err := ctx.Evaluate(expr)
		if err != nil {
			return "", err
		}
		return toString(result), nil
	})
}

// EvaluateBool evaluates an expression and returns a boolean result
func (ctx *Context) EvaluateBool(expr string) (bool, error) {
	// Check if the expression contains the ${{ }} syntax
	if ContainsExpression(expr) {
		// Extract the inner expression
		expressions := ExtractExpressions(expr)
		if len(expressions) > 0 {
			result, err := ctx.Evaluate(expressions[0])
			if err != nil {
				return false, err
			}
			return toBool(result), nil
		}
	}
	
	// If no ${{ }} syntax, evaluate the expression directly
	result, err := ctx.Evaluate(expr)
	if err != nil {
		return false, err
	}
	return toBool(result), nil
}

// evaluator walks through tokens and evaluates expressions
type evaluator struct {
	tokens []Token
	pos    int
	ctx    *Context
}

func (e *evaluator) evaluate() (interface{}, error) {
	return e.parseOr()
}

func (e *evaluator) parseOr() (interface{}, error) {
	left, err := e.parseAnd()
	if err != nil {
		return nil, err
	}

	for e.check(TokenOperator) && e.peek().Value == "||" {
		e.advance() // consume the ||
		right, err := e.parseAnd()
		if err != nil {
			return nil, err
		}
		left = toBool(left) || toBool(right)
	}

	return left, nil
}

func (e *evaluator) parseAnd() (interface{}, error) {
	left, err := e.parseEquality()
	if err != nil {
		return nil, err
	}

	for e.check(TokenOperator) && e.peek().Value == "&&" {
		e.advance() // consume the &&
		right, err := e.parseEquality()
		if err != nil {
			return nil, err
		}
		left = toBool(left) && toBool(right)
	}

	return left, nil
}

func (e *evaluator) parseEquality() (interface{}, error) {
	left, err := e.parseComparison()
	if err != nil {
		return nil, err
	}

	for e.check(TokenOperator) && (e.peek().Value == "==" || e.peek().Value == "!=") {
		op := e.advance().Value
		right, err := e.parseComparison()
		if err != nil {
			return nil, err
		}
		if op == "==" {
			left = equals(left, right)
		} else {
			left = !equals(left, right)
		}
	}

	return left, nil
}

func (e *evaluator) parseComparison() (interface{}, error) {
	left, err := e.parseUnary()
	if err != nil {
		return nil, err
	}

	for e.check(TokenOperator) {
		op := e.peek().Value
		if op != "<" && op != "<=" && op != ">" && op != ">=" {
			break
		}
		e.advance()
		right, err := e.parseUnary()
		if err != nil {
			return nil, err
		}
		leftNum := toNumber(left)
		rightNum := toNumber(right)
		switch op {
		case "<":
			left = leftNum < rightNum
		case "<=":
			left = leftNum <= rightNum
		case ">":
			left = leftNum > rightNum
		case ">=":
			left = leftNum >= rightNum
		}
	}

	return left, nil
}

func (e *evaluator) parseUnary() (interface{}, error) {
	if e.check(TokenOperator) && e.peek().Value == "!" {
		e.advance()
		right, err := e.parseUnary()
		if err != nil {
			return nil, err
		}
		return !toBool(right), nil
	}

	return e.parseCall()
}

func (e *evaluator) parseCall() (interface{}, error) {
	expr, err := e.parsePrimary()
	if err != nil {
		return nil, err
	}

	for {
		if e.match(TokenLeftParen) {
			// Function call
			name, ok := expr.(string)
			if !ok {
				return nil, fmt.Errorf("expected function name")
			}
			expr, err = e.finishCall(name)
			if err != nil {
				return nil, err
			}
		} else if e.match(TokenDot) {
			// Property access
			if !e.check(TokenIdentifier) {
				return nil, fmt.Errorf("expected property name after '.'")
			}
			name := e.advance().Value
			expr = e.getProperty(expr, name)
		} else if e.match(TokenLeftBracket) {
			// Index access
			index, err := e.evaluate()
			if err != nil {
				return nil, err
			}
			if !e.match(TokenRightBracket) {
				return nil, fmt.Errorf("expected ']' after index")
			}
			expr = e.getIndex(expr, index)
		} else {
			break
		}
	}

	return expr, nil
}

func (e *evaluator) finishCall(name string) (interface{}, error) {
	var args []interface{}

	if !e.check(TokenRightParen) {
		for {
			arg, err := e.evaluate()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if !e.match(TokenComma) {
				break
			}
		}
	}

	if !e.match(TokenRightParen) {
		return nil, fmt.Errorf("expected ')' after arguments")
	}

	// Check for context-aware functions first
	if ctxFn, ok := e.ctx.ContextFunctions[name]; ok {
		return ctxFn(e.ctx, args...)
	}

	fn, ok := e.ctx.Functions[name]
	if !ok {
		return nil, fmt.Errorf("unknown function: %s", name)
	}

	return fn(args...)
}

func (e *evaluator) parsePrimary() (interface{}, error) {
	if e.match(TokenNumber) {
		val := e.previous().Value
		if strings.Contains(val, ".") || strings.Contains(val, "e") || strings.Contains(val, "E") {
			f, _ := strconv.ParseFloat(val, 64)
			return f, nil
		}
		i, _ := strconv.ParseInt(val, 10, 64)
		return i, nil
	}

	if e.match(TokenString) {
		return e.previous().Value, nil
	}

	if e.match(TokenIdentifier) {
		name := e.previous().Value
		// Check for keywords
		switch name {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "null":
			return nil, nil
		case "event":
			return e.ctx.Event, nil
		case "env":
			return e.ctx.Env, nil
		case "steps":
			return e.ctx.Steps, nil
		}
		// Return identifier for potential function call
		return name, nil
	}

	if e.match(TokenLeftParen) {
		expr, err := e.evaluate()
		if err != nil {
			return nil, err
		}
		if !e.match(TokenRightParen) {
			return nil, fmt.Errorf("expected ')' after expression")
		}
		return expr, nil
	}

	return nil, fmt.Errorf("unexpected token: %v", e.peek())
}

func (e *evaluator) getProperty(obj interface{}, name string) interface{} {
	if obj == nil {
		return nil
	}

	switch v := obj.(type) {
	case map[string]interface{}:
		return v[name]
	case map[string]string:
		return v[name]
	case map[string]StepContext:
		if step, ok := v[name]; ok {
			return map[string]interface{}{
				"outputs": step.Outputs,
				"outcome": step.Outcome,
			}
		}
		return nil
	default:
		// Use reflection for struct access
		val := reflect.ValueOf(obj)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Kind() == reflect.Struct {
			field := val.FieldByName(name)
			if field.IsValid() {
				return field.Interface()
			}
		}
		return nil
	}
}

func (e *evaluator) getIndex(obj interface{}, index interface{}) interface{} {
	if obj == nil {
		return nil
	}

	switch v := obj.(type) {
	case []interface{}:
		i := int(toNumber(index))
		if i >= 0 && i < len(v) {
			return v[i]
		}
		return nil
	case map[string]interface{}:
		return v[toString(index)]
	case map[string]string:
		return v[toString(index)]
	default:
		return nil
	}
}

func (e *evaluator) match(t TokenType) bool {
	if e.check(t) {
		e.advance()
		return true
	}
	return false
}

func (e *evaluator) check(t TokenType) bool {
	if e.isAtEnd() {
		return false
	}
	return e.peek().Type == t
}

func (e *evaluator) advance() Token {
	if !e.isAtEnd() {
		e.pos++
	}
	return e.previous()
}

func (e *evaluator) peek() Token {
	return e.tokens[e.pos]
}

func (e *evaluator) previous() Token {
	return e.tokens[e.pos-1]
}

func (e *evaluator) isAtEnd() bool {
	return e.peek().Type == TokenEOF
}

// Type conversion helpers
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != ""
	case int64:
		return val != 0
	case float64:
		return val != 0
	default:
		return true
	}
}

func toNumber(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case bool:
		if val {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func equals(a, b interface{}) bool {
	// Handle case-insensitive string comparison
	aStr, aIsStr := a.(string)
	bStr, bIsStr := b.(string)
	if aIsStr && bIsStr {
		return strings.EqualFold(aStr, bStr)
	}
	return reflect.DeepEqual(a, b)
}

// Built-in functions
func builtinContains(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("contains requires 2 arguments")
	}
	search := args[0]
	item := toString(args[1])

	switch v := search.(type) {
	case string:
		return strings.Contains(strings.ToLower(v), strings.ToLower(item)), nil
	case []interface{}:
		for _, elem := range v {
			if strings.EqualFold(toString(elem), item) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, nil
	}
}

func builtinStartsWith(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("startsWith requires 2 arguments")
	}
	str := strings.ToLower(toString(args[0]))
	prefix := strings.ToLower(toString(args[1]))
	return strings.HasPrefix(str, prefix), nil
}

func builtinEndsWith(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("endsWith requires 2 arguments")
	}
	str := strings.ToLower(toString(args[0]))
	suffix := strings.ToLower(toString(args[1]))
	return strings.HasSuffix(str, suffix), nil
}

func builtinFormat(args ...interface{}) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("format requires at least 1 argument")
	}
	format := toString(args[0])
	for i := 1; i < len(args); i++ {
		placeholder := fmt.Sprintf("{%d}", i-1)
		format = strings.ReplaceAll(format, placeholder, toString(args[i]))
	}
	return format, nil
}

func builtinJoin(args ...interface{}) (interface{}, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("join requires 1 or 2 arguments")
	}
	arr, ok := args[0].([]interface{})
	if !ok {
		return toString(args[0]), nil
	}
	sep := ","
	if len(args) == 2 {
		sep = toString(args[1])
	}
	var strs []string
	for _, elem := range arr {
		strs = append(strs, toString(elem))
	}
	return strings.Join(strs, sep), nil
}

func builtinToJSON(args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("toJSON requires 1 argument")
	}
	b, err := json.Marshal(args[0])
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func builtinFromJSON(args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("fromJSON requires 1 argument")
	}
	str := toString(args[0])
	var result interface{}
	if err := json.Unmarshal([]byte(str), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func builtinAlways(args ...interface{}) (interface{}, error) {
	return true, nil
}

func builtinSuccess(ctx *Context, args ...interface{}) (interface{}, error) {
	// success() returns true if no previous steps have failed or been cancelled
	for _, step := range ctx.Steps {
		if step.Outcome == "failure" || step.Outcome == "cancelled" {
			return false, nil
		}
	}
	return true, nil
}

func builtinFailure(ctx *Context, args ...interface{}) (interface{}, error) {
	// failure() returns true if any previous step has failed
	for _, step := range ctx.Steps {
		if step.Outcome == "failure" {
			return true, nil
		}
	}
	return false, nil
}

func builtinCancelled(ctx *Context, args ...interface{}) (interface{}, error) {
	// cancelled() returns true if any previous step has been cancelled
	for _, step := range ctx.Steps {
		if step.Outcome == "cancelled" {
			return true, nil
		}
	}
	return false, nil
}

// readSessionTranscriptRaw reads raw JSONL lines from the context's session dir.
func readSessionTranscriptRaw(ctx *Context) ([]string, error) {
	if ctx.SessionDir == "" {
		return nil, nil
	}
	return session.ReadTranscriptRawFromDir(ctx.SessionDir)
}

// builtinTranscript returns the full transcript or regex-filtered entries as a JSON array string.
// transcript()            — returns all entries
// transcript('pattern')   — returns entries matching the regex pattern
func builtinTranscript(ctx *Context, args ...interface{}) (interface{}, error) {
	lines, err := readSessionTranscriptRaw(ctx)
	if err != nil {
		return "[]", nil
	}

	if len(args) == 0 {
		return wrapLinesAsJSONArray(lines), nil
	}

	if len(args) != 1 {
		return nil, fmt.Errorf("transcript() accepts 0 or 1 arguments")
	}

	pattern := toString(args[0])
	matched, err := session.FilterByRegex(lines, pattern)
	if err != nil {
		return nil, err
	}
	return wrapLinesAsJSONArray(matched), nil
}

// builtinTranscriptSince returns all entries after the last entry matching the regex.
// transcript_since('pattern') — entries after last match
func builtinTranscriptSince(ctx *Context, args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("transcript_since() requires exactly 1 argument (regex pattern)")
	}

	lines, err := readSessionTranscriptRaw(ctx)
	if err != nil {
		return "[]", nil
	}

	pattern := toString(args[0])
	result, err := session.FilterSinceLastMatch(lines, pattern)
	if err != nil {
		return nil, err
	}
	return wrapLinesAsJSONArray(result), nil
}

// builtinTranscriptCount returns the count of entries matching the regex.
// transcript_count('pattern') — count of matches
func builtinTranscriptCount(ctx *Context, args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("transcript_count() requires exactly 1 argument (regex pattern)")
	}

	lines, err := readSessionTranscriptRaw(ctx)
	if err != nil {
		return 0, nil
	}

	pattern := toString(args[0])
	count, err := session.CountMatches(lines, pattern)
	if err != nil {
		return nil, err
	}
	return count, nil
}

// builtinTranscriptLast returns the last entry matching the regex as a JSON string.
// transcript_last('pattern') — last matching entry
func builtinTranscriptLast(ctx *Context, args ...interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("transcript_last() requires exactly 1 argument (regex pattern)")
	}

	lines, err := readSessionTranscriptRaw(ctx)
	if err != nil {
		return "", nil
	}

	pattern := toString(args[0])
	result, err := session.LastMatch(lines, pattern)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// wrapLinesAsJSONArray wraps raw JSONL lines into a JSON array string.
// Each line is already valid JSON, so we compose the array directly.
func wrapLinesAsJSONArray(lines []string) string {
	if len(lines) == 0 {
		return "[]"
	}
	return "[" + strings.Join(lines, ",") + "]"
}
