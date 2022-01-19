package dealer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBinpackRate(t *testing.T) {
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

	s1 := binpack.Rate(gpus1, nil,nil,PolicySpec{})
	s2 := binpack.Rate(gpus2, nil,nil,PolicySpec{})

	assert.True(t, s1 < s2)
}

func TestSpreadRate(t *testing.T) {

	testCases := []struct {
		gpus1            GPUs
		gpus2            GPUs
		firstIsPreferred bool
	}{{
		gpus1: []*GPUResource{
			{
				Percent:      30,
				PercentTotal: 100,
			},
			{
				Percent:      50,
				PercentTotal: 100,
			},
		},
		gpus2: []*GPUResource{
			{
				Percent:      20,
				PercentTotal: 100,
			},
			{
				Percent:      40,
				PercentTotal: 100,
			},
		},
		firstIsPreferred: true,
	}, {
		gpus1: []*GPUResource{
			{
				Percent:      90,
				PercentTotal: 100,
			},
		},
		gpus2: []*GPUResource{
			{
				Percent:      50,
				PercentTotal: 100,
			},
			{
				Percent:      40,
				PercentTotal: 100,
			},
		},
		firstIsPreferred: true,
	}, {
		gpus1: []*GPUResource{
			{
				Percent:      100,
				PercentTotal: 100,
			},
		},
		gpus2: []*GPUResource{
			{
				Percent:      50,
				PercentTotal: 100,
			},
			{
				Percent:      50,
				PercentTotal: 100,
			},
		},
		firstIsPreferred: true,
	}, {
		gpus1: []*GPUResource{
			{
				Percent:      100,
				PercentTotal: 100,
			},
		},
		gpus2: []*GPUResource{
			{
				Percent:      100,
				PercentTotal: 100,
			},
			{
				Percent:      50,
				PercentTotal: 100,
			},
		},
		firstIsPreferred: false,
	},
	}
	spread := &Spread{}

	for _, testCase := range testCases {
		s1 := spread.Rate(testCase.gpus1, nil,nil,PolicySpec{})
		s2 := spread.Rate(testCase.gpus2, nil,nil,PolicySpec{})

		assert.Equal(t, testCase.firstIsPreferred, s1 > s2)
	}
}

func TestBinpackChoose(t *testing.T) {
	testCases := []struct {
		gpus         GPUs
		demand       Demand
		expected     []int
		expectFailed bool
	}{
		{gpus: GPUs{
			{
				Percent:      100,
				PercentTotal: 100,
			},
			{
				Percent:      100,
				PercentTotal: 100,
			},
		},
			demand: Demand{
				{
					Percent:      20,
					PercentTotal: 100,
				},
				{
					Percent:      40,
					PercentTotal: 100,
				},
			},
			expected: []int{0, 0},
		},
		{gpus: GPUs{
			{
				Percent:      20,
				PercentTotal: 100,
			},
			{
				Percent:      100,
				PercentTotal: 100,
			},
		},
			demand: Demand{
				{
					Percent:      20,
					PercentTotal: 100,
				},
				{
					Percent:      40,
					PercentTotal: 100,
				},
			},
			expected: []int{0, 1},
		},
		{gpus: GPUs{
			{
				Percent:      10,
				PercentTotal: 100,
			},
			{
				Percent:      100,
				PercentTotal: 100,
			},
		},
			demand: Demand{
				{
					Percent:      20,
					PercentTotal: 100,
				},
				{
					Percent:      40,
					PercentTotal: 100,
				},
			},
			expected: []int{1, 1},
		},
		{gpus: GPUs{
			{
				Percent:      100,
				PercentTotal: 100,
			},
			{
				Percent:      100,
				PercentTotal: 100,
			},
		},
			demand: Demand{
				{
					Percent:      0,
					PercentTotal: 100,
				},
				{
					Percent:      40,
					PercentTotal: 100,
				},
				{
					Percent:      40,
					PercentTotal: 100,
				},
			},
			expected: []int{-1, 0, 0},
		},
		{gpus: GPUs{
			{
				Percent:      10,
				PercentTotal: 100,
			},
			{
				Percent:      50,
				PercentTotal: 100,
			},
		},
			demand: Demand{
				{
					Percent:      20,
					PercentTotal: 100,
				},
				{
					Percent:      40,
					PercentTotal: 100,
				},
			},
			expected:     nil,
			expectFailed: true,
		},
	}

	binpack := &Binpack{}

	for _, testCase := range testCases {
		indexes, err := binpack.Choose(testCase.gpus, testCase.demand)
		assert.Equal(t, testCase.expectFailed, err != nil)
		if err == nil {
			assert.Equal(t, testCase.expected, indexes)
		}
	}
}

func TestSpreadChoose(t *testing.T) {
	testCases := []struct {
		gpus         GPUs
		demand       Demand
		expected     []int
		expectFailed bool
	}{
		{gpus: GPUs{
			{
				Percent:      100,
				PercentTotal: 100,
			},
			{
				Percent:      100,
				PercentTotal: 100,
			},
		},
			demand: Demand{
				{
					Percent:      20,
					PercentTotal: 100,
				},
				{
					Percent:      40,
					PercentTotal: 100,
				},
			},
			expected: []int{0, 1},
		},
		{gpus: GPUs{
			{
				Percent:      20,
				PercentTotal: 100,
			},
			{
				Percent:      100,
				PercentTotal: 100,
			},
		},
			demand: Demand{
				{
					Percent:      20,
					PercentTotal: 100,
				},
				{
					Percent:      40,
					PercentTotal: 100,
				},
			},
			expected: []int{1, 1},
		},
		{gpus: GPUs{
			{
				Percent:      10,
				PercentTotal: 100,
			},
			{
				Percent:      100,
				PercentTotal: 100,
			},
		},
			demand: Demand{
				{
					Percent:      20,
					PercentTotal: 100,
				},
				{
					Percent:      40,
					PercentTotal: 100,
				},
			},
			expected: []int{1, 1},
		},
		{gpus: GPUs{
			{
				Percent:      100,
				PercentTotal: 100,
			},
			{
				Percent:      100,
				PercentTotal: 100,
			},
		},
			demand: Demand{
				{
					Percent:      0,
					PercentTotal: 100,
				},
				{
					Percent:      40,
					PercentTotal: 100,
				},
				{
					Percent:      40,
					PercentTotal: 100,
				},
			},
			expected: []int{-1, 0, 1},
		},
		{gpus: GPUs{
			{
				Percent:      10,
				PercentTotal: 100,
			},
			{
				Percent:      50,
				PercentTotal: 100,
			},
		},
			demand: Demand{
				{
					Percent:      20,
					PercentTotal: 100,
				},
				{
					Percent:      40,
					PercentTotal: 100,
				},
			},
			expected:     nil,
			expectFailed: true,
		},
	}

	spread := &Spread{}

	for _, testCase := range testCases {
		indexes, err := spread.Choose(testCase.gpus, testCase.demand)
		assert.Equal(t, testCase.expectFailed, err != nil)
		if err == nil {
			assert.Equal(t, testCase.expected, indexes)
		}
	}
}
