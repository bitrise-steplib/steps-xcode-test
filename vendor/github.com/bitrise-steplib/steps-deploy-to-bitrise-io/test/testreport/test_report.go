package testreport

import (
	"encoding/xml"
)

// TestReport is the internal test report structure used to present test results.
type TestReport struct {
	XMLName    xml.Name    `xml:"testsuites"`
	TestSuites []TestSuite `xml:"testsuite"`
}

type TestSuite struct {
	XMLName    xml.Name    `xml:"testsuite"`
	Name       string      `xml:"name,attr"`
	Tests      int         `xml:"tests,attr"`
	Failures   int         `xml:"failures,attr"`
	Errors     int         `xml:"errors,attr"`
	Skipped    int         `xml:"skipped,attr"`
	Assertions int         `xml:"assertions,attr,omitempty"`
	Time       float64     `xml:"time,attr"`
	Timestamp  string      `xml:"timestamp,attr,omitempty"`
	File       string      `xml:"file,attr,omitempty"`
	TestCases  []TestCase  `xml:"testcase,omitempty"`
	TestSuites []TestSuite `xml:"testsuite,omitempty"`
}

type TestCase struct {
	XMLName xml.Name `xml:"testcase"`
	// ConfigurationHash is used to distinguish the same test case runs,
	// performed with different build configurations (e.g., Debug vs. Release) or different devices/simulators
	ConfigurationHash string      `xml:"configuration-hash,attr,omitempty"`
	Name              string      `xml:"name,attr"`
	ClassName         string      `xml:"classname,attr"`
	Assertions        int         `xml:"assertions,attr,omitempty"`
	Time              float64     `xml:"time,attr"`
	File              string      `xml:"file,attr,omitempty"`
	Line              int         `xml:"line,attr,omitempty"`
	Failure           *Failure    `xml:"failure,omitempty"`
	Error             *Error      `xml:"error,omitempty"`
	Skipped           *Skipped    `xml:"skipped,omitempty"`
	Properties        *Properties `xml:"properties,omitempty"`
	SystemOut         *SystemOut  `xml:"system-out,omitempty"`
	SystemErr         *SystemErr  `xml:"system-err,omitempty"`
}

type Failure struct {
	XMLName xml.Name `xml:"failure,omitempty"`
	Type    string   `xml:"type,attr,omitempty"`
	Message string   `xml:"message,attr,omitempty"`
	Value   string   `xml:",chardata"`
}

type Error struct {
	XMLName xml.Name `xml:"error,omitempty"`
	Type    string   `xml:"type,attr,omitempty"`
	Message string   `xml:"message,attr,omitempty"`
	Value   string   `xml:",chardata"`
}

type Skipped struct {
	XMLName xml.Name `xml:"skipped,omitempty"`
	Message string   `xml:"message,attr,omitempty"`
	Value   string   `xml:",chardata"`
}

type Property struct {
	XMLName xml.Name `xml:"property"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:"value,attr"`
}

type Properties struct {
	XMLName  xml.Name   `xml:"properties"`
	Property []Property `xml:"property"`
}

type SystemOut struct {
	XMLName xml.Name `xml:"system-out,omitempty"`
	Value   string   `xml:",chardata"`
}

type SystemErr struct {
	XMLName xml.Name `xml:"system-err,omitempty"`
	Value   string   `xml:",chardata"`
}
