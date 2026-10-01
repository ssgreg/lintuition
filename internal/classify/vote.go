package classify

import (
	"fmt"
	"sort"

	"github.com/ssgreg/lintuition/sdk"
)

// Vote combines the validated answers of several samples of one request into one answer per
// question, or says why they do not agree.
//
// Policy, per question:
//   - choice: the most frequent option must have a strict majority of the samples; its probability
//     is the mean over all samples (a sample that picked another option still contributes its
//     probability for this one). Probabilities of the other options are averaged the same way.
//   - noul: the samples must agree on which side of 0.5 they fall; yes is their mean.
//   - score: the median position.
//
// Agreement is reported separately from probability: three correlated samples agreeing is not a
// calibrated certainty, and the rule's threshold still applies to the mean probability. A failed
// request is not a vote: callers fail the candidate before voting.
func Vote(qs []sdk.Question, samples []map[string]sdk.Answer) (map[string]sdk.Answer, string, error) {
	if len(samples) == 0 {
		return nil, "", fmt.Errorf("no samples")
	}
	if len(samples) == 1 {
		return samples[0], "", nil
	}
	n := len(samples)
	out := map[string]sdk.Answer{}
	for _, q := range qs {
		a := sdk.Answer{QuestionID: q.ID}
		switch q.Kind {
		case sdk.Choice:
			count := map[string]int{}
			sum := map[string]float64{}
			for _, s := range samples {
				x := s[q.ID]
				count[x.Choice]++
				for k, p := range x.Probabilities {
					sum[k] += p
				}
			}
			best, bestN := "", 0
			for _, k := range sortedKeys(count) {
				if count[k] > bestN {
					best, bestN = k, count[k]
				}
			}
			if 2*bestN <= n {
				return nil, fmt.Sprintf("samples disagree on %s: %s", q.ID, tally(count)), nil
			}
			a.Choice = best
			if len(sum) > 0 {
				a.Probabilities = map[string]float64{}
				for k, v := range sum {
					a.Probabilities[k] = v / float64(n)
				}
			}
		case sdk.Noul:
			yes, total := 0, 0.0
			for _, s := range samples {
				v := *s[q.ID].Yes
				total += v
				if v > 0.5 {
					yes++
				}
			}
			if yes != 0 && yes != n {
				return nil, fmt.Sprintf("samples disagree on %s: %d of %d say yes", q.ID, yes, n), nil
			}
			mean := total / float64(n)
			a.Yes = &mean
		case sdk.Score:
			vs := make([]float64, 0, n)
			for _, s := range samples {
				vs = append(vs, *s[q.ID].Score)
			}
			sort.Float64s(vs)
			med := vs[n/2]
			if n%2 == 0 {
				med = (vs[n/2-1] + vs[n/2]) / 2
			}
			a.Score = &med
		}
		out[q.ID] = a
	}
	return out, "", nil
}

func tally(count map[string]int) string {
	s := ""
	for i, k := range sortedKeys(count) {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%s %d", k, count[k])
	}
	return s
}
