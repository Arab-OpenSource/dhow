package pipelinespec

import (
	"errors"

	"github.com/google/cel-go/cel"
)

var ErrContextInvalid = errors.New("expression context is too large or has the wrong type")

func matrixActivation(c combination) map[string]any {
	act := make(map[string]any, len(c))
	for _, v := range c {
		act["matrix."+v.Key] = v.Value.Value()
	}
	return act
}

func activation(c Context, d *declarations, m combination, secret map[string]bool) (map[string]any, error) {
	if !gitFits(c) {
		return nil, ErrContextInvalid
	}
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
	vars, ok := boundedMap(c.Vars, nil, MaxContextVars, MaxContextVarBytes)
	if !ok {
		return nil, ErrContextInvalid
	}
	act["vars"] = vars
	env, ok := boundedMap(c.Env, secret, MaxContextEnv, MaxContextValueBytes)
	if !ok {
		return nil, ErrContextInvalid
	}
	act["env"] = env
	if err := bindInputs(act, c, d); err != nil {
		return nil, err
	}
	if err := bindNeeds(act, c, d); err != nil {
		return nil, err
	}
	return act, nil
}

func gitFits(c Context) bool {
	if len(c.Git.Message) > MaxMessageBytes {
		return false
	}
	for _, s := range []string{
		c.Git.SHA, c.Git.Ref, c.Git.Branch, c.Git.Tag, c.Git.Author, c.Git.Event,
		c.Git.PR.BaseRef, c.Git.PR.HeadRef, c.Project.ID, c.Project.Name,
	} {
		if len(s) > MaxContextTextBytes {
			return false
		}
	}
	return true
}

func boundedMap(in map[string]string, drop map[string]bool, entries, valueBytes int) (map[string]string, bool) {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if drop[k] {
			continue
		}
		if len(k) > MaxContextTextBytes || len(v) > valueBytes {
			return nil, false
		}
		out[k] = v
	}
	return out, len(out) <= entries
}

func bindInputs(act map[string]any, c Context, d *declarations) error {
	for name, t := range d.inputs {
		v, ok := c.Inputs[name]
		isBool := t.IsExactType(cel.BoolType)
		switch {
		case !ok && isBool:
			v = false
		case !ok:
			v = ""
		case isBool:
			if _, ok := v.(bool); !ok {
				return ErrContextInvalid
			}
		default:
			s, ok := v.(string)
			if !ok || len(s) > MaxContextVarBytes {
				return ErrContextInvalid
			}
		}
		act["inputs."+name] = v
	}
	return nil
}

func bindNeeds(act map[string]any, c Context, d *declarations) error {
	for id, isMatrix := range d.needs {
		v, ok := c.Needs[id]
		switch {
		case !ok && isMatrix:
			v = map[string]map[string]string{}
		case !ok:
			v = map[string]string{}
		case isMatrix:
			byKey, ok := v.(map[string]map[string]string)
			if !ok || len(byKey) > MaxMatrixJobs {
				return ErrContextInvalid
			}
			for _, outputs := range byKey {
				if _, ok := boundedMap(outputs, nil, MaxContextOutputs, MaxContextTextBytes); !ok {
					return ErrContextInvalid
				}
			}
		default:
			outputs, ok := v.(map[string]string)
			if !ok {
				return ErrContextInvalid
			}
			if _, ok := boundedMap(outputs, nil, MaxContextOutputs, MaxContextTextBytes); !ok {
				return ErrContextInvalid
			}
		}
		act["needs."+id+".outputs"] = v
	}
	return nil
}
