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

// BenchmarkAnalyze cycles through the commands of both corpora; the budget is 250 µs per command.
func BenchmarkAnalyze(b *testing.B) {
	var commands []string
	for _, r := range loadCorpus(b) {
		commands = append(commands, r.command)
	}
	for _, r := range loadPaths(b) {
		commands = append(commands, r.command)
	}
	c := mustNew(b)
	for i := 0; b.Loop(); i++ {
		c.Analyze(commands[i%len(commands)], corpusHost, "~")
	}
}
