package policy

import (
	"fmt"
	"regexp"
	"strings"
)

type TokenType int

const (
	TOKEN_ROLE TokenType = iota
	TOKEN_USER_ID
	TOKEN_PROJECT_ID
	TOKEN_RULE
	TOKEN_OR
	TOKEN_AND
	TOKEN_NOT
	TOKEN_LPAREN
	TOKEN_RPAREN
	TOKEN_EOF
)

type Token struct {
	Type  TokenType
	Value string
}

type ASTNode struct {
	Type  string // "role", "user_id", "project_id", "rule", "or", "and", "not"
	Value string
	Left  *ASTNode
	Right *ASTNode
}

type Parser struct {
	tokens  []Token
	current int
}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(rule string) (*ASTNode, error) {
	p.tokens = p.tokenize(rule)
	p.current = 0

	if len(p.tokens) == 0 || (len(p.tokens) == 1 && p.tokens[0].Type == TOKEN_EOF) {
		return nil, fmt.Errorf("empty rule")
	}

	ast, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if p.current < len(p.tokens) && p.tokens[p.current].Type != TOKEN_EOF {
		return nil, fmt.Errorf("unexpected tokens after parsing")
	}

	return ast, nil
}

func (p *Parser) tokenize(rule string) []Token {
	var tokens []Token
	rule = strings.TrimSpace(rule)

	rolePattern := regexp.MustCompile(`^role:([a-zA-Z_][a-zA-Z0-9_]*)`)
	userIDPattern := regexp.MustCompile(`^user_id:(%\([^)]+\)s|[^\s)]+)`)
	projectIDPattern := regexp.MustCompile(`^project_id:(%\([^)]+\)s|[^\s)]+)`)
	rulePattern := regexp.MustCompile(`^rule:([a-zA-Z_][a-zA-Z0-9_]*)`)

	for len(rule) > 0 {
		rule = strings.TrimSpace(rule)
		if len(rule) == 0 {
			break
		}

		if strings.HasPrefix(rule, "or ") || rule == "or" {
			tokens = append(tokens, Token{Type: TOKEN_OR})
			rule = rule[2:]
			continue
		}
		if strings.HasPrefix(rule, "and ") || rule == "and" {
			tokens = append(tokens, Token{Type: TOKEN_AND})
			rule = rule[3:]
			continue
		}
		if strings.HasPrefix(rule, "not ") || rule == "not" {
			tokens = append(tokens, Token{Type: TOKEN_NOT})
			rule = rule[3:]
			continue
		}
		if rule[0] == '(' {
			tokens = append(tokens, Token{Type: TOKEN_LPAREN})
			rule = rule[1:]
			continue
		}
		if rule[0] == ')' {
			tokens = append(tokens, Token{Type: TOKEN_RPAREN})
			rule = rule[1:]
			continue
		}

		if match := rolePattern.FindStringSubmatch(rule); match != nil {
			tokens = append(tokens, Token{Type: TOKEN_ROLE, Value: match[1]})
			rule = rule[len(match[0]):]
			continue
		}
		if match := userIDPattern.FindStringSubmatch(rule); match != nil {
			tokens = append(tokens, Token{Type: TOKEN_USER_ID, Value: match[1]})
			rule = rule[len(match[0]):]
			continue
		}
		if match := projectIDPattern.FindStringSubmatch(rule); match != nil {
			tokens = append(tokens, Token{Type: TOKEN_PROJECT_ID, Value: match[1]})
			rule = rule[len(match[0]):]
			continue
		}
		if match := rulePattern.FindStringSubmatch(rule); match != nil {
			tokens = append(tokens, Token{Type: TOKEN_RULE, Value: match[1]})
			rule = rule[len(match[0]):]
			continue
		}

		break
	}

	tokens = append(tokens, Token{Type: TOKEN_EOF})
	return tokens
}

func (p *Parser) parseExpression() (*ASTNode, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}

loop:
	for p.current < len(p.tokens) {
		token := p.tokens[p.current]
		switch token.Type {
		case TOKEN_OR:
			p.current++
			right, err := p.parseTerm()
			if err != nil {
				return nil, err
			}
			left = &ASTNode{Type: "or", Left: left, Right: right}
		case TOKEN_AND:
			p.current++
			right, err := p.parseTerm()
			if err != nil {
				return nil, err
			}
			left = &ASTNode{Type: "and", Left: left, Right: right}
		default:
			break loop
		}
	}

	return left, nil
}

func (p *Parser) parseTerm() (*ASTNode, error) {
	if p.current >= len(p.tokens) {
		return nil, fmt.Errorf("unexpected end of tokens")
	}

	token := p.tokens[p.current]
	p.current++

	switch token.Type {
	case TOKEN_ROLE:
		return &ASTNode{Type: "role", Value: token.Value}, nil
	case TOKEN_USER_ID:
		return &ASTNode{Type: "user_id", Value: token.Value}, nil
	case TOKEN_PROJECT_ID:
		return &ASTNode{Type: "project_id", Value: token.Value}, nil
	case TOKEN_RULE:
		return &ASTNode{Type: "rule", Value: token.Value}, nil
	case TOKEN_NOT:
		node, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		return &ASTNode{Type: "not", Left: node}, nil
	case TOKEN_LPAREN:
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.current >= len(p.tokens) || p.tokens[p.current].Type != TOKEN_RPAREN {
			return nil, fmt.Errorf("expected closing parenthesis")
		}
		p.current++
		return expr, nil
	default:
		return nil, fmt.Errorf("unexpected token type: %d", token.Type)
	}
}
