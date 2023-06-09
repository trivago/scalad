package slack

import (
	"fmt"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	slack "github.com/ashwanthkumar/slack-go-webhook"
)

var (
	useSlack         string
	webhookURL       string
	slackChannel     string
	slackUsername    string
	slackEmoji       string
	compMessagesUp   []string
	compMessagesDown []string
	region           = os.Getenv("NOMAD_REGION")
)

func init() {
	useSlack = os.Getenv("USE_SLACK")
	webhookURL = os.Getenv("SLACK_WEBHOOK")
	slackChannel = os.Getenv("SLACK_CHANNEL")
	slackUsername = os.Getenv("SLACK_USERNAME")
	slackUsername = slackUsername + region
	slackEmoji = ":scalad:"
	if useSlack == "true" {
		if len(webhookURL) == 0 {
			log.Fatal("ENV variable SLACK_WEBHOOK is empty!")
		}
	}
	log.Info("Slack Channel:  ", slackChannel)
	log.Info("Slack Username: ", slackUsername)
	log.Info("Slack emoji:    ", slackEmoji)

}

//StartSlackTicker starts a clock to send a resume of scale events every 10 min.
func StartSlackTicker() {
	tickerB := time.NewTicker(time.Minute * 30)

	go func() {
		for _ = range tickerB.C {
			sendBuffered()
		}
	}()
}

// SendMessage takes a message and send it to a slack channel or user and return an err in case of failure.
func SendMessage(message string) error {
	if useSlack != "true" {
		return nil
	}

	payload := slack.Payload{
		Text:      message,
		Username:  slackUsername,
		Channel:   slackChannel,
		IconEmoji: slackEmoji,
	}
	err := slack.Send(webhookURL, "", payload)
	if err != nil {
		log.Warn("Error sending slack message with err: ", err)
		return fmt.Errorf("Error sending slack message wit err: %v", err)
	}
	return nil
}

// SendMessageTo takes a message and a user with the @ and sends the message to that user. Returns an err in case of failure.
func SendMessageTo(message string, user string) error {
	if useSlack != "true" {
		return nil
	}

	payload := slack.Payload{
		Text:      message,
		Username:  slackUsername,
		Channel:   user,
		IconEmoji: slackEmoji,
	}
	err := slack.Send(webhookURL, "", payload)
	if err != nil {
		log.Warn("Error sending slack message with err: ", err)
		return fmt.Errorf("Error sending slack message wit err: %v", err)
	}
	return nil
}

// MessageBuffered creates a queue of messages and send them to a channel when a ticker expires.
func MessageBuffered(message string, direction string, t time.Time) error {
	if useSlack != "true" {
		return nil
	}
	message = message + `				` + t.Format("2006-01-02 15:04:05")
	if direction == "up" {
		compMessagesUp = append(compMessagesUp, message)
	} else {
		compMessagesDown = append(compMessagesDown, message)
	}
	return nil
}

func sendBuffered() error {
	var message string
	regionUp := strings.ToUpper(region)
	message = `30 min resume of autoscaler in ` + regionUp + `: 
Upscale: 
`
	for _, next := range compMessagesUp {
		message = message + `
` + next
	}
	message = message + `

Downscale: 
`
	for _, next := range compMessagesDown {
		message = message + `
` + next
	}
	message = message + `

`

	payload := slack.Payload{
		Text:      message,
		Username:  slackUsername,
		Channel:   "#scalad-30m",
		IconEmoji: slackEmoji,
	}
	err := slack.Send(webhookURL, "", payload)
	if err != nil {
		log.Warn("Error sending slack message with err: ", err)
		return fmt.Errorf("Error sending slack message wit err: %v", err)
	}
	compMessagesUp = nil
	compMessagesDown = nil
	return nil

}
