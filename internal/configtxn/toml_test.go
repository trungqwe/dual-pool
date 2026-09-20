package configtxn

import (
	"bytes"
	"errors"
	"testing"
)

func TestTOMLSurgicalGoldenShapes(t *testing.T) {
	cases := []struct {
		name   string
		prefix []byte
		nl     string
		bom    bool
	}{
		{"lf", nil, "\n", false}, {"crlf", nil, "\r\n", false}, {"bom-crlf", []byte{0xef, 0xbb, 0xbf}, "\r\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := "# model comment" + tc.nl + "model   =   \"old\"  # keep inline" + tc.nl + "title = \"Định dạng\"" + tc.nl + tc.nl + "[unrelated]" + tc.nl + "array = [1, 2, 3]" + tc.nl + "inline = { x = \"y\" }" + tc.nl
			input := append(append([]byte(nil), tc.prefix...), []byte(body)...)
			p, err := parseDocument(input)
			if err != nil {
				t.Fatal(err)
			}
			out, original, err := p.patch(CodexPlan{}.desired())
			if err != nil {
				t.Fatal(err)
			}
			if tc.bom != bytes.HasPrefix(out, []byte{0xef, 0xbb, 0xbf}) {
				t.Fatal("BOM changed")
			}
			if !bytes.Contains(out, []byte("model   =   \"gpt-6-astra\"  # keep inline")) {
				t.Fatal("scalar formatting changed")
			}
			for _, unchanged := range []string{"# model comment", "title = \"Định dạng\"", "[unrelated]", "array = [1, 2, 3]", "inline = { x = \"y\" }"} {
				if !bytes.Contains(out, []byte(unchanged)) {
					t.Fatalf("unrelated bytes lost")
				}
			}
			parsed, err := parseDocument(out)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := parsed.restore(original)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(restored, input) {
				t.Fatalf("round trip differs\n%s", restored)
			}
		})
	}
}

func TestTOMLExistingProviderAndCatalogGate(t *testing.T) {
	input := []byte("model = \"old\"\nmodel_provider = \"old_provider\"\nmodel_catalog_json = \"keep.json\"\n\n[model_providers.dualpool_codex]\nname = \"User Provider\"\nbase_url = \"http://127.0.0.1:9/v1\"\nwire_api = \"responses\"\nenv_key = \"USER_KEY_NAME\"\n\n[other]\nx = 1\n")
	p, err := parseDocument(input)
	if err != nil {
		t.Fatal(err)
	}
	out, original, err := p.patch(CodexPlan{}.desired())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("model_catalog_json = \"keep.json\"")) {
		t.Fatal("disabled catalog fallback changed")
	}
	next, _ := parseDocument(out)
	restored, err := next.restore(original)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, input) {
		t.Fatal("existing provider did not restore exactly")
	}
}

func TestTOMLUnsupportedShapesFailClosed(t *testing.T) {
	cases := [][]byte{[]byte("model = 'literal'\n"), []byte("\"model\" = \"x\"\n"), []byte("model = \"x\"\nmodel = \"y\"\n"), []byte("[model_providers.dualpool_codex]\nname=\"x\"\nunexpected=\"y\"\n"), []byte("[[model_providers.dualpool_codex]]\nname=\"x\"\n"), []byte("model=\"x\"\r\nother=1\n"), append([]byte{0xff}, []byte("model=\"x\"")...)}
	for i, input := range cases {
		if _, err := parseDocument(input); !errors.Is(err, ErrUnsupportedConfigShape) {
			t.Fatalf("case %d accepted: %v", i, err)
		}
	}
}

func TestTOMLEdgeInsertionAndExactRollback(t *testing.T) {
	for _, input := range [][]byte{
		[]byte("title = \"fixture\""),
		[]byte("# comment only\nname = \"fixture\""),
		[]byte("# comment only"),
	} {
		p, err := parseDocument(input)
		if err != nil {
			t.Fatal(err)
		}
		out, original, err := p.patch(CodexPlan{}.desired())
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parseDocument(out)
		if err != nil || !semanticMatches(parsed.semantic, CodexPlan{}.desired()) {
			t.Fatalf("invalid deterministic insertion: %v", err)
		}
		if bytes.Index(out, []byte("model =")) > bytes.Index(out, []byte("[model_providers.dualpool_codex]")) {
			t.Fatal("top-level scalar inserted inside provider table")
		}
		restored, err := parsed.restore(original)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(restored, input) {
			t.Fatalf("edge round trip differs: %q != %q", restored, input)
		}

		withEdit := append(append([]byte(nil), out...), []byte("\n[unrelated_after]\nvalue = false\n")...)
		parsedEdit, err := parseDocument(withEdit)
		if err != nil {
			t.Fatal(err)
		}
		restoredEdit, err := parsedEdit.restore(original)
		if err != nil {
			t.Fatal(err)
		}
		want := append(append([]byte(nil), input...), []byte("\n[unrelated_after]\nvalue = false\n")...)
		if !bytes.Equal(restoredEdit, want) {
			t.Fatalf("unrelated edge edit lost: %q != %q", restoredEdit, want)
		}
	}
}
