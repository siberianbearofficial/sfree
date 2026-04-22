package manager

import (
	"errors"

	"github.com/example/sfree/api-go/internal/repository"
)

const MaxWeightedSourceWeight = 1000

type SourceSelector interface {
	NextSource(sources []repository.Source) (int, repository.Source, error)
}

type RoundRobinSelector struct {
	next int
}

func (s *RoundRobinSelector) NextSource(sources []repository.Source) (int, repository.Source, error) {
	if len(sources) == 0 {
		return 0, repository.Source{}, errors.New("no sources")
	}
	idx := s.next % len(sources)
	s.next = (s.next + 1) % len(sources)
	return idx, sources[idx], nil
}

type WeightedSelector struct {
	cumulativeWeights []int
	totalWeight       int
	next              int
}

// NewWeightedSelector creates a selector that distributes chunks according to
// per-source weights. The weights map uses source ID hex strings as keys.
// Sources without an entry default to weight 1.
func NewWeightedSelector(sources []repository.Source, weights map[string]int) *WeightedSelector {
	cumulativeWeights := make([]int, 0, len(sources))
	totalWeight := 0
	for _, src := range sources {
		w := weights[src.ID.Hex()]
		if w <= 0 {
			w = 1
		}
		totalWeight += w
		cumulativeWeights = append(cumulativeWeights, totalWeight)
	}
	return &WeightedSelector{cumulativeWeights: cumulativeWeights, totalWeight: totalWeight}
}

func (s *WeightedSelector) NextSource(sources []repository.Source) (int, repository.Source, error) {
	if len(sources) == 0 || s.totalWeight == 0 || len(s.cumulativeWeights) == 0 {
		return 0, repository.Source{}, errors.New("no sources")
	}
	offset := s.next % s.totalWeight
	s.next = (s.next + 1) % s.totalWeight
	idx := 0
	for idx < len(s.cumulativeWeights) && offset >= s.cumulativeWeights[idx] {
		idx++
	}
	if idx >= len(sources) {
		return 0, repository.Source{}, errors.New("source index out of range")
	}
	return idx, sources[idx], nil
}

// SelectorForBucket returns the appropriate SourceSelector based on the
// bucket's configured distribution strategy.
func SelectorForBucket(bucket *repository.Bucket, sources []repository.Source) SourceSelector {
	switch bucket.DistributionStrategy {
	case repository.StrategyWeighted:
		return NewWeightedSelector(sources, bucket.SourceWeights)
	default:
		return &RoundRobinSelector{}
	}
}
