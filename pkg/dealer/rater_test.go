package dealer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBinPack(t *testing.T) {
	gpus1 := []*GPUResource{
		{
			Percent:      30,
			PercentTotal: 100,
		},
		{
			Percent:      50,
			PercentTotal: 100,
		},
	}
	gpus2 := []*GPUResource{
		{
			Percent:      20,
			PercentTotal: 100,
		},
		{
			Percent:      40,
			PercentTotal: 100,
		},
	}

	binpack := &Binpack{}

	s1 := binpack.Rate(gpus1, nil)
	s2 := binpack.Rate(gpus2, nil)

	assert.True(t, s1 < s2)

}
