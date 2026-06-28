package metrics

import "testing"

func TestAppendNewLogLinesDuplicateTailSkipped(t *testing.T) {
	existing := "line one\nline two\nline three\n"
	tail := "line two\nline three\n"

	got := AppendNewLogLines(existing, tail)
	if got != existing {
		t.Errorf("duplicate tail should be skipped\ngot:  %q\nwant: %q", got, existing)
	}
}

func TestAppendNewLogLinesAppendsNewLines(t *testing.T) {
	existing := "line one\nline two\n"
	tail := "line two\nline three\nline four\n"
	want := "line one\nline two\nline three\nline four"

	got := AppendNewLogLines(existing, tail)
	if got != want {
		t.Errorf("AppendNewLogLines = %q, want %q", got, want)
	}
}

func TestAppendNewLogLinesEmptyExisting(t *testing.T) {
	tail := "first line\nsecond line\n"
	got := AppendNewLogLines("", tail)
	if got != tail {
		t.Errorf("got %q, want %q", got, tail)
	}
}

func TestAppendNewLogLinesNoOverlap(t *testing.T) {
	existing := "alpha\nbeta\n"
	tail := "gamma\ndelta\n"
	want := "alpha\nbeta\ngamma\ndelta"

	got := AppendNewLogLines(existing, tail)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
