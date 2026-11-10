package flow

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/chachin/wagot/internal/bot"
)

// Default greetings recognized by the trip agency flow.
var defaultGreetings = []string{"hola", "informacion", "menu"}

// MainMenuButtons returns the standard main menu button reply.
func MainMenuButtons() bot.Buttons {
	return bot.Buttons{
		Body: "¿En qué podemos ayudarte hoy?",
		Buttons: []bot.Button{
			{ID: "trips", Title: "Nuestros viajes"},
			{ID: "contact", Title: "Contacto"},
		},
	}
}

// Flow coordinates the conversational rules.
type Flow struct {
	Greetings       []string
	MainMenu        bot.Buttons
	UnsupportedText string
}

// DefaultFlow returns the default trip agency flow configuration.
func DefaultFlow() *Flow {
	return &Flow{
		Greetings:       defaultGreetings,
		MainMenu:        MainMenuButtons(),
		UnsupportedText: "Por el momento solo puedo leer mensajes de texto.",
	}
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
			if name != "" {
				welcome = fmt.Sprintf("¡Hola, %s! Te damos la bienvenida a Wagot Viajes ✈️", name)
			} else {
				welcome = "¡Hola! Te damos la bienvenida a Wagot Viajes ✈️"
			}
			if err := c.Reply(welcome); err != nil {
				return err
			}
		}
		return c.Send(f.MainMenu)
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
		if norm == g {
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
