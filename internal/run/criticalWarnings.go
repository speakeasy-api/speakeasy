package run

import "regexp"

var criticalWarnings = []*regexp.Regexp{
	regexp.MustCompile(`skipping.*unsupported`),
}

func getCriticalWarnings(warnLogs []string) []string {
	var critical []string

	for _, log := range warnLogs {
		for _, w := range criticalWarnings {
			if w.MatchString(log) {
				critical = append(critical, log)
			}
		}
	}

	return critical
}
