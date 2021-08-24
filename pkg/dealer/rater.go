package dealer

const (
	ScoreMin = 0
	ScoreMax = 100
)

type Rater interface {
	Rate(GPUs, *Plan) int
}

type SampleRater struct {
}

func (sr *SampleRater) Rate(GPUs, *Plan) int {
	return ScoreMax
}