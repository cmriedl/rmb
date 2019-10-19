// Package front provides YAML frontmatter unmarshalling.
package front

import (
	"bytes"

	"gopkg.in/yaml.v2"
)

// Delimiter.
var delim = []byte("---\n")

// Unmarshal parses YAML frontmatter and returns the content. When no
// frontmatter delimiters are present the original content is returned.
func Unmarshal(b []byte, v interface{}) (content []byte, err error) {
	if !bytes.HasPrefix(b, delim) {
		return b, nil
	}

	parts := bytes.SplitN(b, delim, 3)
	content = parts[2]
	err = yaml.UnmarshalStrict(parts[1], v)
	return
}

// Marshal encodes frontmatter to YAML and returns the encoded frontmatter and
// content separated by delimiters.
func Marshal(v interface{}, content []byte) (b []byte, err error) {
	f, err := yaml.Marshal(v)
	b = append(b, delim...)
	b = append(b, f...)
	b = append(b, delim...)
	b = append(b, content...)
	return
}
