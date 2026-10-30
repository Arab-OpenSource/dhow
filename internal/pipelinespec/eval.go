package pipelinespec

import (
	"context"
	"strings"

	"github.com/google/cel-go/cel"
)

type Context struct {
	Git     Git
	Project Project
	Vars    map[string]string
	Inputs  map[string]any
	Needs   map[string]any
	Env     map[string]string
}

type Git struct {
	SHA     string
	Ref     string
	Branch  string
	Tag     string
	IsTag   bool
	Message string
	Author  string
	Event   string
	PR      PullRequest
}

type PullRequest struct {
	Number  int64
	BaseRef string
	HeadRef string
	IsFork  bool
}

type Project struct {
	ID   string
	Name string
}

func matrixActivation(c combination) map[string]any {
	act := make(map[string]any, len(c))
	for _, v := range c {
		act["matrix."+v.Key] = v.Value.Value()
	}
	return act
}

func activation(c Context, d *declarations, m combination) map[string]any {
	act := matrixActivation(m)
	act["git.sha"] = c.Git.SHA
	act["git.ref"] = c.Git.Ref
	act["git.branch"] = c.Git.Branch
	act["git.tag"] = c.Git.Tag
	act["git.is_tag"] = c.Git.IsTag
	act["git.message"] = c.Git.Message
	act["git.author"] = c.Git.Author
	act["git.event"] = c.Git.Event
	act["git.pr.number"] = c.Git.PR.Number
	act["git.pr.base_ref"] = c.Git.PR.BaseRef
	act["git.pr.head_ref"] = c.Git.PR.HeadRef
	act["git.pr.is_fork"] = c.Git.PR.IsFork
	act["project.id"] = c.Project.ID
	act["project.name"] = c.Project.Name
	act["vars"] = orEmpty(c.Vars)
	act["env"] = orEmpty(c.Env)
	for name, t := range d.inputs {
		v, ok := c.Inputs[name]
		if !ok {
			if t.IsExactType(cel.BoolType) {
				v = false
			} else {
				v = ""
			}
		}
		act["inputs."+name] = v
	}
	for id, isMatrix := range d.needs {
		v, ok := c.Needs[id]
		if !ok {
			if isMatrix {
				v = map[string]map[string]string{}
			} else {
				v = map[string]string{}
			}
		}
		act["needs."+id+".outputs"] = v
	}
	return act
}

func orEmpty(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func (p *Plan) EvalGroup(ctx context.Context, c Context) (string, error) {
	if p.Concurrency == nil {
		return "", nil
	}
	g := p.Concurrency.Group
	if g.Expression == "" {
		return g.Literal, nil
	}
	d := &declarations{inputs: p.inputs}
	out, err := evaluate(ctx, scopeGroup, d, g.Expression, activation(c, d, nil))
	if err != nil {
		return "", err
	}
	s, _ := out.(string)
	return s, nil
}

func (r *RunJob) EvalIf(ctx context.Context, c Context) (bool, error) {
	return r.evalBool(ctx, scopeJobIf, r.If, c)
}

func (r *RunJob) EvalStepIf(ctx context.Context, step int, c Context) (bool, error) {
	if step < 0 || step >= len(r.Steps) {
		return false, ErrExpression
	}
	return r.evalBool(ctx, scopeStepIf, r.Steps[step].If, c)
}

func (r *RunJob) evalBool(ctx context.Context, scope exprScope, src string, c Context) (bool, error) {
	if src == "" {
		return true, nil
	}
	out, err := evaluate(ctx, scope, r.decls, src, activation(c, r.decls, r.combo))
	if err != nil {
		return false, err
	}
	b, _ := out.(bool)
	return b, nil
}

func (r *RunJob) EvalValue(ctx context.Context, ref string, c Context) (string, error) {
	src, ok := r.Expressions[ref]
	if !ok {
		return "", ErrExpression
	}
	scope := scopeJobValue
	if strings.HasPrefix(ref, "Steps[") {
		scope = scopeStepValue
	}
	out, err := evaluate(ctx, scope, r.decls, src, activation(c, r.decls, r.combo))
	if err != nil {
		return "", err
	}
	s, _ := out.(string)
	return s, nil
}
