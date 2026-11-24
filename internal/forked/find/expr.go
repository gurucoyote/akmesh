// Local addition, no upstream source: akmesh's own find parses its expressions here.

package find

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// node is one element of the expression tree.
type node interface {
	eval(r *runner, e *entry) bool
	// hasAction reports whether the subtree contains an action, which
	// suppresses the default -print.
	hasAction() bool
}

type andNode struct{ left, right node }

func (n *andNode) eval(r *runner, e *entry) bool {
	// -a short-circuits, so an action behind a failing test never runs:
	// "find . -name x -exec ls ;" lists only the matches.
	if !n.left.eval(r, e) {
		return false
	}
	return n.right.eval(r, e)
}

func (n *andNode) hasAction() bool { return n.left.hasAction() || n.right.hasAction() }

type orNode struct{ left, right node }

func (n *orNode) eval(r *runner, e *entry) bool {
	// -o short-circuits, as in GNU.
	if n.left.eval(r, e) {
		return true
	}
	return n.right.eval(r, e)
}

func (n *orNode) hasAction() bool { return n.left.hasAction() || n.right.hasAction() }

type notNode struct{ inner node }

func (n *notNode) eval(r *runner, e *entry) bool { return !n.inner.eval(r, e) }
func (n *notNode) hasAction() bool               { return n.inner.hasAction() }

// testNode is a predicate without side effects.
type testNode struct {
	fn func(r *runner, e *entry) bool
}

func (n *testNode) eval(r *runner, e *entry) bool { return n.fn(r, e) }
func (n *testNode) hasAction() bool               { return false }

// actionNode prints, deletes or runs something and reports true, so a chain
// behind it still sees a match.
type actionNode struct {
	fn      func(r *runner, e *entry) bool
	deletes bool
}

func (n *actionNode) eval(r *runner, e *entry) bool { return n.fn(r, e) }
func (n *actionNode) hasAction() bool               { return true }

// defaultPrintNode is what find does when the expression carries no action: it
// prints the path when the expression is true. It must not be an -a branch,
// because both sides of -a always run and that would print every entry.
type defaultPrintNode struct{ inner node }

func (n *defaultPrintNode) eval(r *runner, e *entry) bool {
	if n.inner.eval(r, e) {
		r.print(e, false)
	}
	return true
}

func (n *defaultPrintNode) hasAction() bool { return true }

// options are the switches that steer the walk; they are pulled out of the
// argument list before the expression is parsed, because the walk needs them
// before the first entry is evaluated.
type options struct {
	maxDepth int
	minDepth int
	depthAlt bool
	xdev     bool
	follow   bool
}

type parser struct {
	args  []string
	i     int
	roots []string
	opts  options
}

// parse reads the start paths, then the expression.
func parse(args []string) (*parser, node, error) {
	// maxDepth -1 means "no limit": -maxdepth 0 is a real limit that reports
	// the start path alone.
	p := &parser{opts: options{maxDepth: -1}}
	i := 0
	for i < len(args) {
		a := args[i]
		if strings.HasPrefix(a, "-") || a == "!" || a == "(" || a == ")" {
			break
		}
		if a == "--" {
			i++
			for ; i < len(args); i++ {
				p.roots = append(p.roots, args[i])
			}
			break
		}
		p.roots = append(p.roots, a)
		i++
	}
	if len(p.roots) == 0 {
		p.roots = []string{"."}
	}
	p.args = args[i:]
	if err := p.pullOptions(); err != nil {
		return nil, nil, err
	}
	if len(p.args) == 0 {
		// No expression at all prints every path, like GNU.
		return p, &actionNode{fn: func(r *runner, e *entry) bool { r.print(e, false); return true }}, nil
	}
	expr, err := p.parseOr()
	if err != nil {
		return nil, nil, err
	}
	if p.i != len(p.args) {
		return nil, nil, fmt.Errorf("unknown predicate %q", p.args[p.i])
	}
	if !expr.hasAction() {
		expr = &defaultPrintNode{inner: expr}
	}
	return p, expr, nil
}

// pullOptions removes the walk switches from the expression argument list.
func (p *parser) pullOptions() error {
	kept := make([]string, 0, len(p.args))
	for i := 0; i < len(p.args); i++ {
		a := p.args[i]
		switch a {
		case "-maxdepth", "-mindepth":
			if i+1 >= len(p.args) {
				return fmt.Errorf("%s needs a number", a)
			}
			n, err := atoiNonNegative(p.args[i+1])
			if err != nil {
				return fmt.Errorf("%s: invalid number %q", a, p.args[i+1])
			}
			if a == "-maxdepth" {
				p.opts.maxDepth = n
			} else {
				p.opts.minDepth = n
			}
			i++
		case "-depth":
			p.opts.depthAlt = true
		case "-xdev", "-mount":
			p.opts.xdev = true
		case "-follow", "-L":
			p.opts.follow = true
		case "-P":
			// never follow, which is already the default
		case "-H":
			// following only the start paths differs from -L only for a
			// symlink given on the command line; treated as -L here and
			// documented in the usage text
			p.opts.follow = true
		default:
			kept = append(kept, a)
		}
	}
	p.args = kept
	return nil
}

func (p *parser) parseOr() (node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.i < len(p.args) && (p.args[p.i] == "-o" || p.args[p.i] == "-or") {
		p.i++
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &orNode{left, right}
	}
	return left, nil
}

func (p *parser) parseAnd() (node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.i < len(p.args) {
		a := p.args[p.i]
		if a == "-o" || a == "-or" || a == ")" {
			return left, nil
		}
		if a == "-a" || a == "-and" {
			p.i++
		}
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &andNode{left, right}
	}
	return left, nil
}

func (p *parser) parseUnary() (node, error) {
	if p.i >= len(p.args) {
		return nil, errors.New("expression ends too early")
	}
	switch p.args[p.i] {
	case "!", "-not":
		p.i++
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &notNode{inner}, nil
	case "(":
		p.i++
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.i >= len(p.args) || p.args[p.i] != ")" {
			return nil, errors.New("missing )")
		}
		p.i++
		return inner, nil
	case ")":
		return nil, errors.New("unexpected )")
	}
	return p.parsePrimary()
}

// parsePrimary reads one predicate or action with its operands.
func (p *parser) parsePrimary() (node, error) {
	a := p.args[p.i]
	p.i++
	value := func() (string, error) {
		if p.i >= len(p.args) {
			return "", fmt.Errorf("%s needs a value", a)
		}
		v := p.args[p.i]
		p.i++
		return v, nil
	}
	switch a {
	case "-name", "-iname":
		pat, err := value()
		if err != nil {
			return nil, err
		}
		fold := a == "-iname"
		if !fold && !strings.ContainsAny(pat, "*?[") && !strings.Contains(pat, "\\") {
			return &testNode{fn: func(_ *runner, e *entry) bool { return e.name == pat }}, nil
		}
		return &testNode{fn: func(_ *runner, e *entry) bool {
			ok, _ := match(lowerIf(pat, fold), lowerIf(e.name, fold))
			return ok
		}}, nil
	case "-path", "-ipath":
		pat, err := value()
		if err != nil {
			return nil, err
		}
		fold := a == "-ipath"
		return &testNode{fn: func(_ *runner, e *entry) bool {
			ok, _ := match(lowerIf(pat, fold), lowerIf(e.path, fold))
			return ok
		}}, nil
	case "-regex":
		pat, err := value()
		if err != nil {
			return nil, err
		}
		re, err := compileRegex(pat)
		if err != nil {
			return nil, fmt.Errorf("%s %q: %v", a, pat, err)
		}
		return &testNode{fn: func(_ *runner, e *entry) bool { return re.MatchString(e.path) }}, nil
	case "-type":
		letters, err := value()
		if err != nil {
			return nil, err
		}
		check, err := typeCheck(letters)
		if err != nil {
			return nil, err
		}
		return &testNode{fn: func(_ *runner, e *entry) bool { return check(e.info) }}, nil
	case "-size":
		spec, err := value()
		if err != nil {
			return nil, err
		}
		check, err := sizeCheck(spec)
		if err != nil {
			return nil, err
		}
		return &testNode{fn: func(_ *runner, e *entry) bool { return check(e.info) }}, nil
	case "-empty":
		return &testNode{fn: func(r *runner, e *entry) bool { return r.isEmpty(e) }}, nil
	case "-newer":
		file, err := value()
		if err != nil {
			return nil, err
		}
		ref, err := statTime(file)
		if err != nil {
			return nil, err
		}
		return &testNode{fn: func(_ *runner, e *entry) bool { return e.info.ModTime().After(ref) }}, nil
	case "-mtime", "-mmin":
		spec, err := value()
		if err != nil {
			return nil, err
		}
		cmp, err := numberCompare(spec)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", a, err)
		}
		unit := 24 * time.Hour
		if a == "-mmin" {
			unit = time.Minute
		}
		return &testNode{fn: func(r *runner, e *entry) bool {
			// GNU counts whole units, rounded down.
			return cmp(int64(r.now.Sub(e.info.ModTime()) / unit))
		}}, nil
	case "-user", "-group":
		v, err := value()
		if err != nil {
			return nil, err
		}
		check, err := ownerCheck(a == "-user", v)
		if err != nil {
			return nil, err
		}
		return &testNode{fn: func(_ *runner, e *entry) bool { return check(e.info) }}, nil
	case "-perm", "-mode":
		spec, err := value()
		if err != nil {
			return nil, err
		}
		check, err := permCheck(a, spec)
		if err != nil {
			return nil, err
		}
		return &testNode{fn: func(_ *runner, e *entry) bool { return check(e.info) }}, nil
	case "-print":
		return &actionNode{fn: func(r *runner, e *entry) bool { r.print(e, false); return true }}, nil
	case "-printf":
		format, err := value()
		if err != nil {
			return nil, err
		}
		if err := checkPrintf(format); err != nil {
			return nil, err
		}
		return &actionNode{fn: func(r *runner, e *entry) bool { r.printf(format, e); return true }}, nil
	case "-print0":
		return &actionNode{fn: func(r *runner, e *entry) bool { r.print(e, true); return true }}, nil
	case "-delete":
		return &actionNode{deletes: true, fn: func(r *runner, e *entry) bool { r.delete(e); return true }}, nil
	case "-quit":
		return &actionNode{fn: func(r *runner, e *entry) bool { r.quit = true; return true }}, nil
	case "-exec":
		var argv []string
		plus := false
		for {
			if p.i >= len(p.args) {
				return nil, errors.New("-exec needs a command and a terminating ';'")
			}
			v := p.args[p.i]
			p.i++
			if v == ";" {
				break
			}
			if v == "+" {
				plus = true
				break
			}
			argv = append(argv, v)
		}
		if len(argv) == 0 {
			return nil, errors.New("-exec needs a command")
		}
		if plus {
			return &actionNode{fn: func(r *runner, e *entry) bool { r.execPlus(argv, e); return true }}, nil
		}
		return &actionNode{fn: func(r *runner, e *entry) bool { r.exec(argv, e); return true }}, nil
	}
	return nil, fmt.Errorf("unknown predicate %q", a)
}

func lowerIf(s string, fold bool) string {
	if fold {
		return strings.ToLower(s)
	}
	return s
}

// match is filepath.Match, but a pattern without wildcards also has to match a
// name that contains no special characters at all (GNU compares literally when
// the pattern has no metacharacter, which filepath.Match already does).
func match(pattern, name string) (bool, error) {
	ok, err := pathMatch(pattern, name)
	if err != nil {
		return false, err
	}
	return ok, nil
}

var _ = exec.Command
