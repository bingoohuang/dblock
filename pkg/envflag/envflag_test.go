package envflag

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test1(t *testing.T) {
	assert.Equal(t, "JAPAN_CANADA_AUSTRALIA", ToSnakeCase("JapanCanadaAustralia"))
	assert.Equal(t, "JAPAN_CANADA_AUSTRALIA", ToSnakeCase("JapanCanadaAUSTRALIA"))
	assert.Equal(t, "JAPAN_CANADA_AUSTRALIA", ToSnakeCase("JAPANCanadaAUSTRALIA"))
	assert.Equal(t, "JAPAN125_CANADA130_AUSTRALIA150", ToSnakeCase("Japan125Canada130Australia150"))
}
