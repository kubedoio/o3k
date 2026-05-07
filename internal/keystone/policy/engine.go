package policy

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type Engine struct {
	policies map[string]string
	cache    *Cache
	mu       sync.RWMutex
}

func NewEngine() *Engine {
	return &Engine{
		policies: make(map[string]string),
		cache:    NewCache(5 * time.Minute),
	}
}

func (e *Engine) LoadPolicy(policies map[string]string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.policies = policies
	e.cache.Flush()
}

func (e *Engine) Enforce(rule string, target, credentials map[string]interface{}) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if cached, ok := e.cache.Get(rule, target, credentials); ok {
		return cached
	}

	ruleExpr, ok := e.policies[rule]
	if !ok {
		return false
	}

	parser := NewParser()
	ast, err := parser.Parse(ruleExpr)
	if err != nil {
		log.Error().Err(err).Str("rule", rule).Msg("failed to parse policy rule")
		return false
	}

	result := e.evaluate(ast, target, credentials)
	e.cache.Set(rule, target, credentials, result)
	return result
}

func (e *Engine) evaluate(node *ASTNode, target, credentials map[string]interface{}) bool {
	switch node.Type {
	case "role":
		roles, ok := credentials["roles"].([]string)
		if !ok {
			return false
		}
		for _, r := range roles {
			if r == node.Value {
				return true
			}
		}
		return false

	case "user_id":
		targetUserID := e.interpolate(node.Value, target, credentials)
		credUserID, _ := credentials["user_id"].(string)
		return credUserID == targetUserID

	case "project_id":
		targetProjectID := e.interpolate(node.Value, target, credentials)
		credProjectID, _ := credentials["project_id"].(string)
		return credProjectID == targetProjectID

	case "rule":
		ruleExpr, ok := e.policies[node.Value]
		if !ok {
			return false
		}
		parser := NewParser()
		ast, err := parser.Parse(ruleExpr)
		if err != nil {
			return false
		}
		return e.evaluate(ast, target, credentials)

	case "or":
		return e.evaluate(node.Left, target, credentials) || e.evaluate(node.Right, target, credentials)

	case "and":
		return e.evaluate(node.Left, target, credentials) && e.evaluate(node.Right, target, credentials)

	case "not":
		return !e.evaluate(node.Left, target, credentials)
	}

	return false
}

var interpolateRe = regexp.MustCompile(`%\((target|credentials)\.([a-zA-Z_]+)\)s`)

func (e *Engine) interpolate(template string, target, credentials map[string]interface{}) string {
	return interpolateRe.ReplaceAllStringFunc(template, func(match string) string {
		parts := interpolateRe.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		var data map[string]interface{}
		if parts[1] == "target" {
			data = target
		} else {
			data = credentials
		}

		value, ok := data[parts[2]]
		if !ok {
			return ""
		}
		return fmt.Sprintf("%v", value)
	})
}
