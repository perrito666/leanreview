package editor

import (
	"fmt"
	"regexp"
	"strings"
)

// TemplateContext carries the metadata rendered into the editor header comment.
// Fields left empty are omitted (e.g. PullRequest in local mode).
type TemplateContext struct {
	Repository  string
	PullRequest string
	File        string
	Lines       string
	Side        string
	ReplyTo     string
}

// BuildTemplate renders the initial editor buffer: an HTML-comment header with
// context, followed by any prior body. The header is stripped after editing, so
// the reviewer never has to delete it manually.
func BuildTemplate(ctx TemplateContext, body string) string {
	var h strings.Builder
	h.WriteString("<!--\n")
	writeField(&h, "Repository", ctx.Repository)
	writeField(&h, "Pull request", ctx.PullRequest)
	writeField(&h, "File", ctx.File)
	writeField(&h, "Lines", ctx.Lines)
	writeField(&h, "Side", ctx.Side)
	writeField(&h, "Reply to", ctx.ReplyTo)
	h.WriteString("Write your comment below. This comment block is removed automatically.\n")
	h.WriteString("-->\n\n")
	h.WriteString(body)
	return h.String()
}

func writeField(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) != "" {
		fmt.Fprintf(b, "%s: %s\n", label, value)
	}
}

var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)

// CleanTemplate removes HTML comment blocks and trims surrounding whitespace,
// yielding the reviewer's note.
func CleanTemplate(s string) string {
	s = htmlComment.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
