package flow

import (
	_ "embed"
	"fmt"
	"strings"
	"unicode"

	"github.com/chachin/wagot/internal/bot"
)

//go:embed default_flow.json
var defaultFlowJSON []byte

// Step represents an action taken when a user selects an option.
type Step struct {
	Reply            bot.Reply
	FollowUpMainMenu bool
}

// Flow coordinates the conversational rules.
type Flow struct {
	Greetings       []string
	WelcomeTemplate string
	WelcomeFallback string
	MainMenu        bot.Buttons
	UnsupportedText string
	Steps           map[string]Step
}

// DefaultFlow returns the default trip agency flow configuration.
func DefaultFlow() *Flow {
	fl, err := LoadJSON(defaultFlowJSON)
	if err != nil {
		panic(fmt.Sprintf("embedded default flow is invalid: %v", err))
	}
	return fl
}

// NewHandler returns a bot.Handler using DefaultFlow.
func NewHandler() bot.Handler {
	return DefaultFlow().Handle
}

// Handle evaluates an incoming message and emits replies through the Conversation Context.
func (f *Flow) Handle(c *bot.Context) error {
	switch c.Message.Type {
	case bot.MessageTypeText:
		if f.isGreeting(c.Message.Text) {
			name := strings.TrimSpace(c.Contact.Profile.Name)
			var welcome string
			if name != "" && f.WelcomeTemplate != "" && strings.Contains(f.WelcomeTemplate, "%s") {
				welcome = fmt.Sprintf(f.WelcomeTemplate, name)
			} else if f.WelcomeFallback != "" {
				welcome = f.WelcomeFallback
			} else if f.WelcomeTemplate != "" {
				w := strings.ReplaceAll(f.WelcomeTemplate, ", %s", "")
				w = strings.ReplaceAll(w, "%s, ", "")
				w = strings.ReplaceAll(w, "%s", "")
				welcome = strings.TrimSpace(w)
			}
			if welcome != "" {
				if err := c.Reply(welcome); err != nil {
					return err
				}
			}
		}
		return c.Send(f.MainMenu)
	case bot.MessageTypeInteractive, bot.MessageTypeButton:
		step, ok := f.Steps[c.Message.SelectionID]
		if !ok {
			return c.Send(f.MainMenu)
		}
		if err := c.Send(step.Reply); err != nil {
			return err
		}
		if step.FollowUpMainMenu {
			return c.Send(f.MainMenu)
		}
		return nil
	case bot.MessageTypeReaction, bot.MessageTypeSystem:
		return nil
	default:
		if err := c.React("👀"); err != nil {
			return err
		}
		unsupportedText := f.UnsupportedText
		if unsupportedText == "" {
			unsupportedText = "Por el momento solo puedo leer mensajes de texto."
		}
		return c.Reply(unsupportedText)
	}
}

func (f *Flow) isGreeting(text string) bool {
	norm := normalizeText(text)
	for _, g := range f.Greetings {
		if norm == normalizeText(g) {
			return true
		}
	}
	return false
}

func normalizeText(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		switch r {
		case 'á', 'Á':
			b.WriteRune('a')
		case 'é', 'É':
			b.WriteRune('e')
		case 'í', 'Í':
			b.WriteRune('i')
		case 'ó', 'Ó':
			b.WriteRune('o')
		case 'ú', 'Ú', 'ü', 'Ü':
			b.WriteRune('u')
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return strings.TrimSpace(b.String())
}
