package pipelinespec_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Khaymah/dhow/internal/pipelinespec"
	"github.com/Khaymah/dhow/internal/pipelinespec/pipelinespectest"
)

func TestValidationErrors(t *testing.T) {
	for _, tc := range pipelinespectest.Invalid() {
		t.Run(tc.Name, func(t *testing.T) {
			_, issues := pipelinespec.Check([]byte(tc.Source))
			codes := []string{}
			for _, i := range issues {
				if i.Severity == pipelinespec.SeverityError {
					codes = append(codes, i.Code)
				}
				assert.NotEmpty(t, i.Message)
				assert.Equal(t, strings.ToUpper(i.Code), i.Code)
			}
			assert.Contains(t, codes, tc.Code, "%v", issues)
		})
	}
}
