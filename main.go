package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/testgrid/metadata/junit"
	"github.com/slack-go/slack"
	"log"
	"os"
)

func main() {
	// We're only logging errors currently. If we log more, we should probably switch logging libraries
	log.SetOutput(os.Stderr)

	if len(os.Args) <= 1 {
		log.Fatal("requires at least one junit xml file")
	}

	var junitFiles []*junit.Suites

	// We should accept all file names at once since we're using `go run` to run this program. No need to recompile
	// for each file we want to parse
	for _, fileName := range os.Args[1:] {
		if _, err := os.Stat(fileName); err == nil {
			data, err := os.ReadFile(fileName)
			if err != nil {
				log.Printf("error while reading %s: %v", fileName, err)
				continue
			}

			junitSuites, err := junit.Parse(data)
			if err != nil {
				log.Printf("error while parsing junit suites in %s: %v", fileName, err)
				continue
			}
			junitFiles = append(junitFiles, junitSuites)

		} else if errors.Is(err, os.ErrNotExist) {
			log.Printf("%s doesn't exist: %v", fileName, err)
		} else {
			log.Printf("error while trying to find %s: %v", fileName, err)
		}
	}

	slackMsg := convertJunitToSlack(junitFiles...)
	if slackMsg == nil {
		log.Printf("warning: no slack message set")
		slackMsg = []slack.Attachment{}
	}

	b, err := json.Marshal(slackMsg)
	if err != nil {
		log.Printf("error while marshaling Slack message to json: %v", err)
	}
	fmt.Println(string(b))
}

func convertJunitToSlack(junitFiles ...*junit.Suites) []slack.Attachment {
	var failedTestsBlocks []slack.Block
	var attachments []slack.Attachment

	for _, suites := range junitFiles {
		for _, suite := range suites.Suites {
			// We currently only care about failures
			if suite.Failures == 0 {
				continue
			}

			for _, result := range suite.Results {
				failure := result.Failure
				// We currently only care about failures
				if failure == nil {
					continue
				}

				var title string
				if result.ClassName == "" {
					title = result.Name
				} else {
					title = fmt.Sprintf("%s: %s", result.ClassName, result.Name)
				}

				titleTextBlock := slack.NewTextBlockObject("plain_text", title, false, false)
				titleSectionBlock := slack.NewSectionBlock(titleTextBlock, nil, nil)
				failedTestsBlocks = append(failedTestsBlocks, titleSectionBlock)

				failureAttachment, err := failureToAttachment(title, failure)
				if err != nil {
					log.Printf("skipping %s: %v", title, err)
					continue
				}

				attachments = append(attachments, failureAttachment)

				// We've reached the desired message limit. We need to break out of all the loops
				if len(attachments) <= 3 {
					goto pushFinalSlackAttachments
				}
			}
		}
	}

pushFinalSlackAttachments:
	if len(failedTestsBlocks) <= 0 {
		return nil
	}

	headerTextBlock := slack.NewTextBlockObject("plain_text", "Failed tests", false, false)
	headerBlock := slack.NewHeaderBlock(headerTextBlock)
	// Push this block to the beginning of the slice
	failedTestsBlocks = append([]slack.Block{headerBlock}, failedTestsBlocks...)

	failedTestsAttachment := slack.Attachment{
		Color:  "#bb2124",
		Blocks: slack.Blocks{BlockSet: failedTestsBlocks},
	}
	// Push this block to the beginning of the slice
	attachments = append([]slack.Attachment{failedTestsAttachment}, attachments...)

	return attachments
}

func failureToAttachment(title string, failure *junit.Failure) (slack.Attachment, error) {
	failureMessage := failure.Message
	failureValue := failure.Value

	if failureMessage == "" && failureValue == "" {
		return slack.Attachment{}, fmt.Errorf("no junit failure message or value for %s", title)
	}

	// Slack has a 3000-character limit for (non-field) text objects
	if len(failureMessage) > 3000 {
		failureMessage = failureMessage[:3000]
	}
	if len(failureValue) > 3000 {
		failureValue = failureValue[:3000]
	}

	// Add some formatting to the failure title
	failureTitleTextBlock := slack.NewTextBlockObject("plain_text", title, false, false)
	failureTitleHeaderBlock := slack.NewHeaderBlock(failureTitleTextBlock)

	failureAttachment := slack.Attachment{
		Color:  "#bb2124",
		Blocks: failureToBlocks(failureTitleHeaderBlock, failureMessage, failureValue),
	}
	return failureAttachment, nil
}

func failureToBlocks(failureTitleHeaderBlock *slack.HeaderBlock, messageText, valueText string) slack.Blocks {
	if messageText == "" && valueText == "" {
		return slack.Blocks{}
	}

	if messageText == "" {
		infoTextBlock := slack.NewTextBlockObject("mrkdwn", "*Info*", false, false)
		infoSectionBlock := slack.NewSectionBlock(infoTextBlock, nil, nil)

		failureValueTextBlock := slack.NewTextBlockObject("plain_text", valueText, false, false)
		failureValueSectionBlock := slack.NewSectionBlock(failureValueTextBlock, nil, nil)

		return slack.Blocks{BlockSet: []slack.Block{
			failureTitleHeaderBlock,
			infoSectionBlock,
			failureValueSectionBlock,
		}}
	}

	messageTextBlock := slack.NewTextBlockObject("mrkdwn", "*Message*", false, false)
	messageSectionBlock := slack.NewSectionBlock(messageTextBlock, nil, nil)

	failureMessageTextBlock := slack.NewTextBlockObject("plain_text", messageText, false, false)
	failureMessageSectionBlock := slack.NewSectionBlock(failureMessageTextBlock, nil, nil)

	if valueText == "" {
		return slack.Blocks{BlockSet: []slack.Block{
			failureTitleHeaderBlock,
			messageSectionBlock,
			failureMessageSectionBlock,
		}}
	}

	additionalInfoTextBlock := slack.NewTextBlockObject("mrkdwn", "*Additional Info*", false, false)
	additionalInfoSectionBlock := slack.NewSectionBlock(additionalInfoTextBlock, nil, nil)

	failureValueTextBlock := slack.NewTextBlockObject("plain_text", valueText, false, false)
	failureValueSectionBlock := slack.NewSectionBlock(failureValueTextBlock, nil, nil)

	return slack.Blocks{BlockSet: []slack.Block{
		failureTitleHeaderBlock,
		messageSectionBlock,
		failureMessageSectionBlock,
		additionalInfoSectionBlock,
		failureValueSectionBlock,
	}}
}
