package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func renderYAMLDocuments(manifests []map[string]any) string {
	documents := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		documents = append(documents, strings.Join(renderYAMLValue(manifest, 0), "\n"))
	}
	return strings.Join(documents, "\n---\n")
}

func renderYAMLValue(value any, indent int) []string {
	switch typed := value.(type) {
	case map[string]any:
		return renderYAMLMapLines(typed, indent)
	case []any:
		return renderYAMLListLines(typed, indent)
	default:
		return []string{strings.Repeat(" ", indent) + renderYAMLScalar(typed)}
	}
}

func renderYAMLMapLines(values map[string]any, indent int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	prefix := strings.Repeat(" ", indent)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		value := values[key]
		switch typed := value.(type) {
		case map[string]any:
			lines = append(lines, prefix+key+":")
			lines = append(lines, renderYAMLMapLines(typed, indent+2)...)
		case []any:
			lines = append(lines, prefix+key+":")
			lines = append(lines, renderYAMLListLines(typed, indent+2)...)
		default:
			lines = append(lines, prefix+key+": "+renderYAMLScalar(typed))
		}
	}

	return lines
}

func renderYAMLListLines(values []any, indent int) []string {
	prefix := strings.Repeat(" ", indent)
	lines := make([]string, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			itemLines := renderYAMLMapLines(typed, indent+2)
			if len(itemLines) == 0 {
				lines = append(lines, prefix+"- {}")
				continue
			}
			first := strings.TrimPrefix(itemLines[0], strings.Repeat(" ", indent+2))
			lines = append(lines, prefix+"- "+first)
			lines = append(lines, itemLines[1:]...)
		case []any:
			lines = append(lines, prefix+"-")
			lines = append(lines, renderYAMLListLines(typed, indent+2)...)
		default:
			lines = append(lines, prefix+"- "+renderYAMLScalar(typed))
		}
	}
	return lines
}

func renderYAMLScalar(value any) string {
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return `""`
		}
		if needsYAMLQuoting(typed) {
			return strconv.Quote(typed)
		}
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func needsYAMLQuoting(value string) bool {
	return strings.ContainsAny(value, ":{}[]&,#?|-<>=!%@\\\"'") || strings.Contains(value, " ")
}
