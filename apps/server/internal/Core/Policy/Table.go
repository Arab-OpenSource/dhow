package policy

import "net/http"

type Scope uint8

const (
	ScopePublic Scope = iota + 1
	ScopeAuthenticated
	ScopeOrganization
	ScopeInstance
)

type Rule struct {
	Method   string
	Path     string
	Scope    Scope
	Action   Action
	OrgParam string
}

func Public(method, path string) Rule {
	return Rule{Method: method, Path: path, Scope: ScopePublic}
}

func Authenticated(method, path string, action Action) Rule {
	return Rule{Method: method, Path: path, Scope: ScopeAuthenticated, Action: action}
}

func Organization(method, path string, action Action, orgParam string) Rule {
	return Rule{Method: method, Path: path, Scope: ScopeOrganization, Action: action, OrgParam: orgParam}
}

func Instance(method, path string, action Action) Rule {
	return Rule{Method: method, Path: path, Scope: ScopeInstance, Action: action}
}

func (r Rule) Key() string { return r.Method + " " + r.Path }

func (r Rule) Valid() bool {
	switch r.Scope {
	case ScopePublic:
		return r.Action == ""
	case ScopeAuthenticated:
		return r.Action == SessionManageSelf
	case ScopeOrganization:
		return r.Action != "" && r.Action != SessionManageSelf && !IsInstanceAction(r.Action) && r.OrgParam != ""
	case ScopeInstance:
		return IsInstanceAction(r.Action) && r.OrgParam == ""
	default:
		return false
	}
}

func (r Rule) Unsafe() bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

type Table struct {
	rules map[string]Rule
}

func NewTable(rules ...Rule) *Table {
	t := &Table{rules: make(map[string]Rule, len(rules))}
	for _, r := range rules {
		t.rules[r.Key()] = r
	}
	return t
}

func (t *Table) Lookup(method, path string) (Rule, bool) {
	r, ok := t.rules[method+" "+path]
	if !ok || !r.Valid() {
		return Rule{}, false
	}
	return r, true
}
