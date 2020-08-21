package main

import (
	"flag"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/log"
	"github.com/promlabs/promql-compliance-tester/comparer"
	"github.com/promlabs/promql-compliance-tester/config"
	"github.com/promlabs/promql-compliance-tester/output"
	"github.com/promlabs/promql-compliance-tester/testcases"
)

func newPromAPI(url string) (v1.API, error) {
	client, err := api.NewClient(api.Config{
		Address: url,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "creating Prometheus API client for %q: %v", url)
	}

	return v1.NewAPI(client), nil
}

func main() {
	configFile := flag.String("config-file", "promql-compliance-tester.yml", "The path to the configuration file.")
	outputFormat := flag.String("output-format", "text", "The comparison output format. Valid values: [text, html, json]")
	outputHTMLTemplate := flag.String("output-html-template", "./output/example-output.html", "The HTML template to use when using HTML as the output format.")
	outputPassing := flag.Bool("output-passing", false, "Whether to also include passing test cases in the output.")
	flag.Parse()

	var outp output.Outputter
	switch *outputFormat {
	case "text":
		outp = output.Text
	case "html":
		var err error
		outp, err = output.HTML(*outputHTMLTemplate)
		if err != nil {
			log.Fatalf("Error reading output HTML template: %v", err)
		}
	case "json":
		outp = output.JSON
	default:
		log.Fatalf("Invalid output format %q", *outputFormat)
	}

	cfg, err := config.LoadFromFile(*configFile)
	if err != nil {
		log.Fatalf("Error loading configuration file: %v", err)
	}

	refAPI, err := newPromAPI(cfg.ReferenceTargetConfig.QueryURL)
	if err != nil {
		log.Fatalf("Error creating reference API: %v", err)
	}
	testAPI, err := newPromAPI(cfg.TestTargetConfig.QueryURL)
	if err != nil {
		log.Fatalf("Error creating test API: %v", err)
	}

	comp := comparer.Comparer{
		RefAPI:      refAPI,
		TestAPI:     testAPI,
		QueryTweaks: cfg.QueryTweaks,
	}

	// Expand all placeholder variations in the templated test cases.
	end := time.Now().Add(-2 * time.Minute)
	start := end.Add(-10 * time.Minute)
	resolution := 10 * time.Second
	expandedTestCases := testcases.ExpandTestCases(cfg.TestCases, cfg.QueryTweaks, start, end, resolution)

	progressBar := pb.StartNew(len(expandedTestCases))
	results := make([]*comparer.Result, 0, len(cfg.TestCases))
	for _, tc := range expandedTestCases {
		res, err := comp.Compare(tc)
		if err != nil {
			log.Fatalf("Error running comparison: %v", err)
		}
		progressBar.Increment()
		results = append(results, res)
	}
	progressBar.Finish()

	outp(results, *outputPassing, cfg.QueryTweaks)
}
