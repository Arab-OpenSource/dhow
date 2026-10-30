package pipelinespec

const (
	digest = "61bb7a95b0f3650f3039c0fcabe07674458cf0d13afa51aade6188bab8215b07"
	img    = "alpine@sha256:" + digest
)

func pipeline(jobs string) string {
	return "version: 1\non:\n  push: {}\njobs:\n" + jobs
}

func job(body string) string {
	return pipeline("  a:\n    image: " + img + "\n" + body)
}

func runJob(stepExtra string) string {
	return job("    steps:\n      - run: make\n" + stepExtra)
}

func issueCodes(issues []Issue, severity Severity) []string {
	out := []string{}
	for _, i := range issues {
		if i.Severity == severity {
			out = append(out, i.Code)
		}
	}
	return out
}
