package provider

import (
	"testing"

	_ "embed"

	expect "github.com/sudosu404/go-utils/testing"
)

//go:embed all_fields.yaml
var testAllFieldsYAML []byte

func TestFile(t *testing.T) {
	_, err := validate(testAllFieldsYAML)
	expect.NoError(t, err)
}
