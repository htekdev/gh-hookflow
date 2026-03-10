package ai

import (
	"fmt"
	"strings"
)

// GenerateWorkflowResult contains the generated workflow
type GenerateWorkflowResult struct {
	Name        string
	Description string
	YAML        string
}

// buildWorkflowPrompt creates the full prompt with schema context
func buildWorkflowPrompt(userPrompt string) string {
	return fmt.Sprintf(promptTemplate, userPrompt)
}

var promptTemplate = "You are an expert at creating hookflow workflow files. " +
"These workflows run locally during AI agent editing sessions to enforce governance and quality gates.\n\n" +
"Generate a workflow YAML file for the following requirement:\n%s\n\n" +
"## Workflow Schema\n\n" +
"The workflow must follow this structure:\n\n" +
"- name: (required) Human-readable workflow name\n" +
"- description: (optional) What the workflow does\n" +
"- on: (required) Trigger configuration - can be:\n" +
"  - hooks: Match hook type (preToolUse, postToolUse)\n" +
"  - tool: Match specific tool with args patterns\n" +
"  - file: Match file events with paths and types\n" +
"  - commit: Match commit events with paths/message patterns\n" +
"  - push: Match push events with branches/tags\n" +
"- blocking: (optional, default true) Whether to block on failure\n" +
"- env: (optional) Environment variables\n" +
"- steps: (required) List of steps to execute\n" +
"  - name: Step name\n" +
"  - if: Conditional expression\n" +
"  - run: Shell command to execute\n" +
"  - env: Step-specific environment variables\n\n" +
"## Trigger Examples\n\n" +
"File trigger (use 'types' for actions like edit/create/delete):\n" +
"on:\n" +
"  file:\n" +
"    paths:\n" +
"      - '**/*.env*'\n" +
"    types:\n" +
"      - edit\n" +
"      - create\n\n" +
"Commit trigger:\n" +
"on:\n" +
"  commit:\n" +
"    paths:\n" +
"      - 'src/**'\n\n" +
"Tool trigger:\n" +
"on:\n" +
"  tool:\n" +
"    name: edit\n" +
"    args:\n" +
"      path: '**/secrets/**'\n\n" +
"## Expression Syntax\n\n" +
	"Use ${{ }} for expressions:\n" +
	"- ${{ event.file.path }} - File path being edited\n" +
	"- ${{ event.tool.args.path }} - Tool argument (for tool triggers)\n" +
	"- ${{ event.commit.message }} - Commit message\n" +
	"- ${{ contains(event.file.path, '.env') }} - Check if path contains .env\n" +
	"- ${{ endsWith(event.file.path, '.ts') }} - Check file extension\n\n" +
"## Output Requirements\n\n" +
"1. Output ONLY the YAML workflow file content\n" +
"2. Start with --- (YAML document separator)\n" +
"3. Include descriptive name and description\n" +
"4. Use appropriate triggers for the requirement\n" +
"5. For file triggers, use 'types' field (not 'actions')\n" +
"6. Include clear step names\n" +
	"7. Add exit 1 to block/deny the action when needed\n\n" +
	"Generate the workflow now:"

// extractYAML finds YAML content in the response
func extractYAML(response string) string {
	tripleBacktick := string([]byte{96, 96, 96})
	yamlMarker := tripleBacktick + "yaml"

	if idx := strings.Index(response, yamlMarker); idx != -1 {
		start := idx + len(yamlMarker)
		end := strings.Index(response[start:], tripleBacktick)
		if end != -1 {
			return strings.TrimSpace(response[start : start+end])
		}
	}

	if idx := strings.Index(response, tripleBacktick); idx != -1 {
		start := idx + 3
		if newline := strings.Index(response[start:], "\n"); newline != -1 {
			start += newline + 1
		}
		end := strings.Index(response[start:], tripleBacktick)
		if end != -1 {
			return strings.TrimSpace(response[start : start+end])
		}
	}

	if idx := strings.Index(response, "---"); idx != -1 {
		return strings.TrimSpace(response[idx:])
	}

	if idx := strings.Index(response, "name:"); idx != -1 {
		return strings.TrimSpace(response[idx:])
	}

	return ""
}

// extractWorkflowName extracts the name from YAML
func extractWorkflowName(yaml string) string {
	lines := strings.Split(yaml, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			name := strings.TrimPrefix(line, "name:")
			name = strings.TrimSpace(name)
			name = strings.Trim(name, "\"'")
			return name
		}
	}
	return "generated-workflow"
}