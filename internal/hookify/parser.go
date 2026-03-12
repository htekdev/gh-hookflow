package hookify

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseRule reads a hookify markdown file and parses it into a Rule.
func ParseRule(filePath string) (*Rule, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read hookify rule file %s: %w", filePath, err)
	}
	rule, err := ParseRuleFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse hookify rule file %s: %w", filePath, err)
	}
	rule.FilePath = filePath
	return rule, nil
}

// ParseRuleFromBytes parses hookify markdown content (YAML frontmatter + markdown body) into a Rule.
func ParseRuleFromBytes(data []byte) (*Rule, error) {
	content := string(data)

	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var rule Rule
	if err := yaml.Unmarshal([]byte(frontmatter), &rule); err != nil {
		return nil, fmt.Errorf("invalid YAML frontmatter: %w", err)
	}

	rule.Message = strings.TrimSpace(body)

	if err := validateRule(&rule); err != nil {
		return nil, err
	}

	// Auto-convert simple pattern to a condition
	if rule.Pattern != "" && len(rule.Conditions) == 0 {
		rule.Conditions = []Condition{patternToCondition(rule.Event, rule.Pattern)}
	}

	return &rule, nil
}

// splitFrontmatter splits a markdown document into YAML frontmatter and body.
// Frontmatter is delimited by "---" on its own line at the start and end.
func splitFrontmatter(content string) (string, string, error) {
	content = strings.TrimSpace(content)

	if !strings.HasPrefix(content, "---") {
		return "", "", fmt.Errorf("hookify rule must start with YAML frontmatter (---)")
	}

	// Find the closing ---
	rest := content[3:]
	// Skip the newline after opening ---
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	closingIdx := strings.Index(rest, "\n---")
	if closingIdx == -1 {
		return "", "", fmt.Errorf("hookify rule missing closing frontmatter delimiter (---)")
	}

	frontmatter := rest[:closingIdx]
	body := rest[closingIdx+4:] // skip past "\n---"

	// Skip the newline after closing ---
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	} else if len(body) > 1 && body[0] == '\r' && body[1] == '\n' {
		body = body[2:]
	}

	return frontmatter, body, nil
}

// validateRule checks that a parsed rule has all required fields and valid values.
func validateRule(rule *Rule) error {
	if rule.Name == "" {
		return fmt.Errorf("hookify rule missing required field: name")
	}
	if rule.Event == "" {
		return fmt.Errorf("hookify rule %q missing required field: event", rule.Name)
	}
	if !ValidEvents[rule.Event] {
		return fmt.Errorf("hookify rule %q has invalid event type: %q", rule.Name, rule.Event)
	}
	if rule.Action != "" && !ValidActions[rule.Action] {
		return fmt.Errorf("hookify rule %q has invalid action: %q", rule.Name, rule.Action)
	}
	if rule.Pattern == "" && len(rule.Conditions) == 0 {
		return fmt.Errorf("hookify rule %q must have either pattern or conditions", rule.Name)
	}
	if rule.Pattern != "" && len(rule.Conditions) > 0 {
		return fmt.Errorf("hookify rule %q cannot have both pattern and conditions", rule.Name)
	}

	for i, cond := range rule.Conditions {
		if cond.Field == "" {
			return fmt.Errorf("hookify rule %q condition[%d] missing required field: field", rule.Name, i)
		}
		if !ValidFields[cond.Field] {
			return fmt.Errorf("hookify rule %q condition[%d] has invalid field: %q", rule.Name, i, cond.Field)
		}
		if cond.Operator == "" {
			return fmt.Errorf("hookify rule %q condition[%d] missing required field: operator", rule.Name, i)
		}
		if !ValidOperators[cond.Operator] {
			return fmt.Errorf("hookify rule %q condition[%d] has invalid operator: %q", rule.Name, i, cond.Operator)
		}
		if cond.Pattern == "" {
			return fmt.Errorf("hookify rule %q condition[%d] missing required field: pattern", rule.Name, i)
		}
		// content field concatenates map values in non-deterministic order;
		// only order-independent operators are safe.
		if cond.Field == FieldContent && contentPositionalOperators[cond.Operator] {
			return fmt.Errorf("hookify rule %q condition[%d]: operator %q is not supported with field %q (map iteration order is non-deterministic; use contains, not_contains, or regex_match instead)", rule.Name, i, cond.Operator, cond.Field)
		}
	}

	return nil
}

// patternToCondition converts a simple pattern string into a Condition,
// choosing the field based on the event type (following hookify's convention).
func patternToCondition(event, pattern string) Condition {
	field := FieldContent // default
	switch event {
	case EventBash:
		field = FieldCommand
	case EventFile:
		field = FieldFilePath
	}
	return Condition{
		Field:    field,
		Operator: OpRegexMatch,
		Pattern:  pattern,
	}
}
