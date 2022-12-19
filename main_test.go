package main

import (
	_ "embed"
	"encoding/json"
	"github.com/GoogleCloudPlatform/testgrid/metadata/junit"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	//go:embed testdata/message-sample.xml
	messageSample string
	//go:embed testdata/value-sample.xml
	valueSample string
	//go:embed testdata/combined-sample.xml
	combinedSample string

	//go:embed testdata/message-expected.json
	messageExpected []byte
	//go:embed testdata/value-expected.json
	valueExpected []byte
	//go:embed testdata/combined-expected.json
	combinedExpected []byte
)

func TestConstructSlackMessage(t *testing.T) {
	samples := []string{messageSample, valueSample, combinedSample}
	expectations := [][]byte{messageExpected, valueExpected, combinedExpected}

	assert.Len(t, samples, len(expectations), "There are different amounts of samples and expected files. This a problem with the test rather than the code")

	for i := 0; i < len(samples); i++ {
		suites, err := junit.Parse([]byte(samples[i]))
		assert.NoError(t, err, "If this fails, it probably indicates a problem with the sample junit report rather than the code")
		assert.NotNil(t, suites, "If this fails, it probably indicates a problem with the sample junit report rather than the code")

		blocks := convertJunitToSlack(suites)
		b, err := json.Marshal(blocks)
		assert.NoError(t, err)

		assert.Equal(t, expectations[i], b)
	}
}
