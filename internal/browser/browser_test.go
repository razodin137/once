package browser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenRequiresURL(t *testing.T) {
	require.Error(t, Open(""))
}
