package workbench

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"
)

// RequirementsDocument binds the rendered text to committed repository bytes.
type RequirementsDocument struct {
	PrototypeStatus string                `json:"prototype_status,omitempty"`
	CheckoutPath    string                `json:"checkout_path,omitempty"`
	Ref             OneSpecArtifactRef    `json:"ref"`
	Content         string                `json:"content"`
	Prototype       *RequirementsDocument `json:"prototype,omitempty"`
	Verified        bool                  `json:"verified"`
	Error           string                `json:"error,omitempty"`
}

func (d RequirementsDocument) Validate(prototype bool) error {
	if d.PrototypeStatus != "" && d.PrototypeStatus != "none" && d.PrototypeStatus != "preparing" && d.PrototypeStatus != "ready" && d.PrototypeStatus != "failed" {
		return errors.New("invalid prototype status")
	}
	if d.PrototypeStatus == "ready" && d.Prototype == nil {
		return errors.New("ready prototype requires its HTML document")
	}
	if !utf8.ValidString(d.Content) || strings.TrimSpace(d.Content) == "" || len(d.Content) > 512*1024 {
		return errors.New("provide a UTF-8 document of at most 512 KiB")
	}
	ext := ".md"
	if prototype {
		ext = ".html"
	}
	if !strings.HasSuffix(strings.ToLower(d.Ref.Path), ext) || !strings.HasPrefix(d.Ref.Path, "onespec/") {
		return errors.New("document must be a Markdown file or prototype HTML under onespec/")
	}
	digest := sha256.Sum256([]byte(d.Content))
	if hex.EncodeToString(digest[:]) != d.Ref.Digest {
		return errors.New("document content does not match its SHA256 digest")
	}
	if d.Prototype != nil {
		if prototype {
			return errors.New("a prototype cannot contain another prototype")
		}
		return d.Prototype.Validate(true)
	}
	return nil
}

func (a OneSpecArtifact) DocumentReady() bool {
	return a.Document == nil || (a.Document.Verified && a.Document.PrototypeStatus != "preparing" && a.Document.PrototypeStatus != "failed" && (a.Document.Prototype == nil || a.Document.Prototype.Verified))
}

func SameRequirementDocuments(a, b *RequirementsDocument) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Content == b.Content && a.PrototypeStatus == b.PrototypeStatus && SameRequirementDocuments(a.Prototype, b.Prototype)
}

func RequirementItemDocumentChanged(previous, current *RequirementsDocument, section string) bool {
	if previous == nil || current == nil || !SameRequirementDocuments(previous.Prototype, current.Prototype) || previous.PrototypeStatus != current.PrototypeStatus {
		return true
	}
	if previous.Content == current.Content {
		return false
	}
	before, foundBefore := requirementSection(previous.Content, section)
	after, foundAfter := requirementSection(current.Content, section)
	return !foundBefore || !foundAfter || before != after
}

func requirementSection(content, section string) (string, bool) {
	section = strings.TrimSpace(strings.TrimLeft(section, "#"))
	if section == "" {
		return "", false
	}
	lines := strings.Split(content, "\n")
	start, level := -1, 0
	for index, line := range lines {
		text := strings.TrimLeft(line, "#")
		depth := len(line) - len(text)
		if depth < 1 || depth > 6 || !strings.HasPrefix(text, " ") {
			continue
		}
		if start >= 0 && depth <= level {
			return strings.Join(lines[start:index], "\n"), true
		}
		if start < 0 && strings.TrimSpace(text) == section {
			start, level = index, depth
		}
	}
	if start < 0 {
		return "", false
	}
	return strings.Join(lines[start:], "\n"), true
}
