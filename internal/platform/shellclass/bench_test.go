package shellclass

import "testing"

// BenchmarkClassify cycles through the corpus commands; the budget is 200 µs per command.
func BenchmarkClassify(b *testing.B) {
	rows := loadCorpus(b)
	c := mustNew(b)
	for i := 0; b.Loop(); i++ {
		c.Classify(rows[i%len(rows)].command, corpusHost)
	}
}
