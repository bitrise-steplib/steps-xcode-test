package selector

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"time"
)

type junitCase struct {
	Time     string    `xml:"time,attr"`
	Failures []xmlNode `xml:"failure"`
	Errors   []xmlNode `xml:"error"`
	Skipped  []xmlNode `xml:"skipped"`
	// Surefire-style retry reports: a passing case with these failed before passing.
	Flaky []xmlNode `xml:"flakyFailure"`
	Rerun []xmlNode `xml:"rerunFailure"`
}

type xmlNode struct{}

type junitSuite struct {
	Cases  []junitCase  `xml:"testcase"`
	Suites []junitSuite `xml:"testsuite"`
}

// JUnitOutcome sums JUnit XML files into one unit's outcome. Skipped cases are not
// counted; a case with flakyFailure or rerunFailure entries that passed is flaky.
func JUnitOutcome(key string, files []string) (UnitOutcome, error) {
	out := UnitOutcome{Key: key}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return out, err
		}
		var root junitSuite
		if err := xml.Unmarshal(data, &root); err != nil {
			return out, fmt.Errorf("parse %s: %w", f, err)
		}
		addSuite(&out, root)
	}
	return out, nil
}

func addSuite(out *UnitOutcome, s junitSuite) {
	for _, c := range s.Cases {
		if len(c.Skipped) > 0 {
			continue
		}
		out.Tests++
		switch {
		case len(c.Failures) > 0 || len(c.Errors) > 0:
			out.Failed++
		case len(c.Flaky) > 0 || len(c.Rerun) > 0:
			out.Flaky++
		}
		if secs, err := strconv.ParseFloat(c.Time, 64); err == nil {
			out.Duration += time.Duration(secs * float64(time.Second))
		}
	}
	for _, child := range s.Suites {
		addSuite(out, child)
	}
}
