package configtxn

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	toml "github.com/pelletier/go-toml/v2"
)

const maxConfigBytes = 1 << 20

type documentMeta struct {
	bom     bool
	newline string
}

type span struct{ start, end int }
type byteEdit struct {
	span
	replacement []byte
}

type parsedDocument struct {
	raw             []byte
	meta            documentMeta
	semantic        map[string]value
	scalars         map[string]span
	providerScalars map[string]span
	table           span
	firstTable      int
}

var (
	scalarLine     = regexp.MustCompile(`^(\s*)(model|model_provider|model_catalog_json)(\s*=\s*)("(?:[^"\\]|\\.)*")(\s*(?:#.*)?)$`)
	tableHeader    = regexp.MustCompile(`^\s*\[model_providers\.dualpool_codex\]\s*(?:#.*)?$`)
	anyTableHeader = regexp.MustCompile(`^\s*\[.*\]\s*(?:#.*)?$`)
	providerLine   = regexp.MustCompile(`^\s*(name|base_url|wire_api|env_key)\s*=\s*("(?:[^"\\]|\\.)*")\s*(?:#.*)?$`)
)

func parseDocument(input []byte) (parsedDocument, error) {
	if len(input) == 0 || len(input) > maxConfigBytes || !utf8.Valid(input) {
		return parsedDocument{}, ErrUnsupportedConfigShape
	}
	meta := documentMeta{newline: "\n"}
	data := input
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		meta.bom = true
		data = data[3:]
	}
	if bytes.Contains(data, []byte("\r\n")) {
		meta.newline = "\r\n"
		if bytes.Contains(bytes.ReplaceAll(data, []byte("\r\n"), nil), []byte("\n")) {
			return parsedDocument{}, ErrUnsupportedConfigShape
		}
	} else if bytes.Contains(data, []byte("\r")) {
		return parsedDocument{}, ErrUnsupportedConfigShape
	}
	var decoded map[string]any
	if err := toml.Unmarshal(data, &decoded); err != nil {
		return parsedDocument{}, ErrUnsupportedConfigShape
	}
	p := parsedDocument{raw: append([]byte(nil), input...), meta: meta, semantic: map[string]value{}, scalars: map[string]span{}, providerScalars: map[string]span{}, firstTable: -1}
	offset := 0
	if meta.bom {
		offset = 3
	}
	lines := splitLinesWithOffsets(data, offset)
	inProvider := false
	providerStart := -1
	providerEnd := -1
	provider := map[string]string{}
	for _, line := range lines {
		text := strings.TrimSuffix(strings.TrimSuffix(string(line.data), "\n"), "\r")
		trimmedLine := strings.TrimSpace(text)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}
		if anyTableHeader.MatchString(text) {
			if p.firstTable < 0 {
				p.firstTable = line.start
			}
			if inProvider {
				providerEnd = line.start
				inProvider = false
			}
			if tableHeader.MatchString(text) {
				if providerStart >= 0 {
					return parsedDocument{}, ErrUnsupportedConfigShape
				}
				inProvider = true
				providerStart = line.start
			}
			continue
		}
		if inProvider {
			m := providerLine.FindStringSubmatchIndex(text)
			if m == nil {
				return parsedDocument{}, ErrUnsupportedConfigShape
			}
			key := text[m[2]:m[3]]
			if _, exists := provider[key]; exists {
				return parsedDocument{}, ErrUnsupportedConfigShape
			}
			decodedValue, err := decodeStringLiteral(text[m[4]:m[5]])
			if err != nil {
				return parsedDocument{}, ErrUnsupportedConfigShape
			}
			provider[key] = decodedValue
			p.providerScalars[key] = span{line.start + m[4], line.start + m[5]}
			continue
		}
		if m := scalarLine.FindStringSubmatchIndex(text); m != nil {
			key := text[m[4]:m[5]]
			if _, exists := p.scalars[key]; exists {
				return parsedDocument{}, ErrUnsupportedConfigShape
			}
			p.scalars[key] = span{line.start + m[8], line.start + m[9]}
			v, err := decodeStringLiteral(text[m[8]:m[9]])
			if err != nil {
				return parsedDocument{}, ErrUnsupportedConfigShape
			}
			p.semantic[key] = stringValue(v)
		} else if containsOwnedToken(text) {
			return parsedDocument{}, ErrUnsupportedConfigShape
		}
	}
	if inProvider {
		providerEnd = len(input)
	}
	if providerStart >= 0 {
		if len(provider) != 4 {
			return parsedDocument{}, ErrUnsupportedConfigShape
		}
		p.table = span{providerStart, providerEnd}
		p.semantic[keyProviderTable] = mapValue(provider)
	}
	// Cross-check mature parser semantics so the constrained locator cannot miss aliases.
	for _, key := range []string{keyModel, keyModelProvider, keyCatalog} {
		if raw, ok := decoded[key]; ok {
			s, ok := raw.(string)
			if !ok || !p.semantic[key].exists || p.semantic[key].str != s {
				return parsedDocument{}, ErrUnsupportedConfigShape
			}
		} else if p.semantic[key].exists {
			return parsedDocument{}, ErrUnsupportedConfigShape
		}
	}
	if mp, ok := decoded["model_providers"].(map[string]any); ok {
		if raw, ok := mp["dualpool_codex"]; ok {
			table, ok := raw.(map[string]any)
			if !ok || !p.semantic[keyProviderTable].exists {
				return parsedDocument{}, ErrUnsupportedConfigShape
			}
			if len(table) != len(provider) {
				return parsedDocument{}, ErrUnsupportedConfigShape
			}
			for k, rawValue := range table {
				s, ok := rawValue.(string)
				if !ok || provider[k] != s {
					return parsedDocument{}, ErrUnsupportedConfigShape
				}
			}
		}
	} else if _, ok := decoded["model_providers"]; ok {
		return parsedDocument{}, ErrUnsupportedConfigShape
	}
	return p, nil
}

type lineOffset struct {
	data  []byte
	start int
}

func splitLinesWithOffsets(data []byte, base int) []lineOffset {
	var out []lineOffset
	for len(data) > 0 {
		i := bytes.IndexByte(data, '\n')
		n := len(data)
		if i >= 0 {
			n = i + 1
		}
		out = append(out, lineOffset{data[:n], base})
		base += n
		data = data[n:]
	}
	return out
}
func containsOwnedToken(s string) bool {
	return strings.Contains(s, "model_provider") || strings.Contains(s, "model_catalog_json") || regexp.MustCompile(`\bmodel\b`).MatchString(s) || strings.Contains(s, "dualpool_codex")
}
func decodeStringLiteral(s string) (string, error) {
	var v struct {
		V string `toml:"v"`
	}
	if err := toml.Unmarshal([]byte("v = "+s), &v); err != nil {
		return "", err
	}
	return v.V, nil
}
func encodeString(v string) string { return fmt.Sprintf("%q", v) }

func (p parsedDocument) patch(desired map[string]value) ([]byte, map[string]value, error) {
	original := map[string]value{}
	for key := range desired {
		if v, ok := p.semantic[key]; ok {
			original[key] = v
		} else {
			original[key] = absentValue()
		}
	}
	var edits []byteEdit
	var insertScalars []string
	for _, key := range []string{keyModel, keyModelProvider, keyCatalog} {
		desiredValue, wanted := desired[key]
		if !wanted {
			continue
		}
		if existing, ok := p.scalars[key]; ok {
			edits = append(edits, byteEdit{existing, []byte(encodeString(desiredValue.str))})
		} else {
			insertScalars = append(insertScalars, key+" = "+encodeString(desiredValue.str)+p.meta.newline)
		}
	}
	combineTailInsert := len(insertScalars) > 0 && p.firstTable < 0 && p.table.end <= p.table.start
	if len(insertScalars) > 0 && !combineTailInsert {
		pos := p.firstTable
		if pos < 0 {
			pos = len(p.raw)
		}
		prefix := ""
		if pos > 0 && !bytes.HasSuffix(p.raw[:pos], []byte(p.meta.newline)) {
			prefix = p.meta.newline
		}
		edits = append(edits, byteEdit{span{pos, pos}, []byte(prefix + strings.Join(insertScalars, ""))})
	}
	tableDesired := desired[keyProviderTable]
	tableText := renderProviderTable(tableDesired.table, p.meta.newline)
	if p.table.end > p.table.start {
		for _, key := range []string{"name", "base_url", "wire_api", "env_key"} {
			edits = append(edits, byteEdit{p.providerScalars[key], []byte(encodeString(tableDesired.table[key]))})
		}
	} else {
		prefix := ""
		if len(p.raw) > 0 {
			// Always own exactly one separator newline together with a newly
			// inserted table so rollback can remove it byte-for-byte.
			prefix = p.meta.newline
		}
		insert := prefix + tableText
		if combineTailInsert {
			scalarPrefix := ""
			if len(p.raw) > 0 && !bytes.HasSuffix(p.raw, []byte(p.meta.newline)) {
				scalarPrefix = p.meta.newline
			}
			insert = scalarPrefix + strings.Join(insertScalars, "") + p.meta.newline + tableText
		}
		edits = append(edits, byteEdit{span{len(p.raw), len(p.raw)}, []byte(insert)})
	}
	return applyEdits(p.raw, edits), original, nil
}

func (p parsedDocument) restore(original map[string]value) ([]byte, error) {
	var edits []byteEdit
	allInserted := !original[keyProviderTable].exists
	earliestInserted := len(p.raw)
	for _, key := range []string{keyModel, keyModelProvider, keyCatalog} {
		orig, managed := original[key]
		if managed && orig.exists {
			allInserted = false
		}
		if managed && !orig.exists {
			if current, ok := p.scalars[key]; ok {
				line := wholeLine(p.raw, current)
				if line.start < earliestInserted {
					earliestInserted = line.start
				}
			}
		}
	}
	if allInserted && earliestInserted < len(p.raw) && p.table.end > p.table.start && earliestInserted < p.table.start {
		start := earliestInserted
		if start >= len(p.meta.newline) && bytes.Equal(p.raw[start-len(p.meta.newline):start], []byte(p.meta.newline)) {
			start -= len(p.meta.newline)
		}
		var replacement []byte
		if p.table.end < len(p.raw) {
			replacement = []byte(p.meta.newline)
		}
		return applyEdits(p.raw, []byteEdit{{span{start, p.table.end}, replacement}}), nil
	}
	for _, key := range []string{keyModel, keyModelProvider, keyCatalog} {
		orig, managed := original[key]
		if !managed {
			continue
		}
		currentSpan, exists := p.scalars[key]
		if !exists {
			return nil, ErrRollbackConflict
		}
		if orig.exists {
			edits = append(edits, byteEdit{currentSpan, []byte(encodeString(orig.str))})
		} else {
			line := wholeLine(p.raw, currentSpan)
			edits = append(edits, byteEdit{line, nil})
		}
	}
	origTable := original[keyProviderTable]
	if p.table.end <= p.table.start {
		return nil, ErrRollbackConflict
	}
	if origTable.exists {
		for _, key := range []string{"name", "base_url", "wire_api", "env_key"} {
			field, ok := p.providerScalars[key]
			if !ok {
				return nil, ErrRollbackConflict
			}
			edits = append(edits, byteEdit{field, []byte(encodeString(origTable.table[key]))})
		}
	} else {
		start := p.table.start
		if start >= len(p.meta.newline) && bytes.Equal(p.raw[start-len(p.meta.newline):start], []byte(p.meta.newline)) {
			start -= len(p.meta.newline)
		}
		edits = append(edits, byteEdit{span{start, p.table.end}, nil})
	}
	return applyEdits(p.raw, edits), nil
}

func wholeLine(data []byte, s span) span {
	start := bytes.LastIndex(data[:s.start], []byte("\n")) + 1
	endRel := bytes.Index(data[s.end:], []byte("\n"))
	end := len(data)
	if endRel >= 0 {
		end = s.end + endRel + 1
	}
	return span{start, end}
}
func applyEdits(data []byte, edits []byteEdit) []byte {
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := append([]byte(nil), data...)
	for _, e := range edits {
		out = append(append(append([]byte(nil), out[:e.start]...), e.replacement...), out[e.end:]...)
	}
	return out
}
func renderProviderTable(values map[string]string, nl string) string {
	var b strings.Builder
	b.WriteString("[model_providers.dualpool_codex]" + nl)
	for _, k := range []string{"name", "base_url", "wire_api", "env_key"} {
		b.WriteString(k + " = " + encodeString(values[k]) + nl)
	}
	return b.String()
}
func equalValue(a, b value) bool {
	if a.exists != b.exists || a.str != b.str || len(a.table) != len(b.table) {
		return false
	}
	for k, v := range a.table {
		if b.table[k] != v {
			return false
		}
	}
	return true
}
