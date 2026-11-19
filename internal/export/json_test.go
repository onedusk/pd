package export

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportDecomposition_NotFound(t *testing.T) {
	data, err := ExportDecomposition(t.TempDir(), "missing")
	require.Error(t, err)
	assert.Nil(t, data)
	assert.Contains(t, err.Error(), "missing")
}
