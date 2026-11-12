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

// Step represents an action taken when a user selects an option.
type Step struct {
	Reply            bot.Reply
	FollowUpMainMenu bool
}

// DefaultSteps returns the built-in trip and contact step definitions.
func DefaultSteps() map[string]Step {
	return map[string]Step{
		"trips": {
			Reply: bot.List{
				Body:   "Explora nuestros paquetes turísticos más populares. Elige un destino para conocer detalles y fotografías:",
				Button: "Ver destinos",
				Items: []bot.ListItem{
					{
						ID:          "trip_cancun",
						Title:       "Cancún, México",
						Description: "Playas paradisíacas, sol y arqueología maya.",
					},
					{
						ID:          "trip_machu_picchu",
						Title:       "Machu Picchu, Perú",
						Description: "Historia viva, cultura andina y naturaleza.",
					},
					{
						ID:          "trip_buenos_aires",
						Title:       "Buenos Aires, Arg.",
						Description: "Tango, arquitectura clásica y gastronomía.",
					},
				},
			},
			FollowUpMainMenu: false,
		},
		"trip_cancun": {
			Reply: bot.Image{
				URL:     "https://images.unsplash.com/photo-1510097467424-192d713fd8b2",
				Caption: "🏖️ Cancún, México: Disfruta de 5 días y 4 noches en resort todo incluido frente al mar Caribe con tour guiado a Chichén Itzá.",
			},
			FollowUpMainMenu: true,
		},
		"trip_machu_picchu": {
			Reply: bot.Image{
				URL:     "https://images.unsplash.com/photo-1526392060635-9d6019884377",
				Caption: "🏔️ Machu Picchu, Perú: Paquete de 6 días que incluye pasajes en tren panorámico, entrada a la ciudadela inca y estadía en Cusco.",
			},
			FollowUpMainMenu: true,
		},
		"trip_buenos_aires": {
			Reply: bot.Image{
				URL:     "https://images.unsplash.com/photo-1589909202802-8f4aadce1849",
				Caption: "🍷 Buenos Aires, Argentina: 4 días de cultura y tango en San Telmo y Recoleta con cena-show y recorrido gastronómico.",
			},
			FollowUpMainMenu: true,
		},
		"contact": {
			Reply: bot.Text{
				Body: "📍 *Wagot Viajes - Atención al Cliente*\n\n" +
					"Estamos listos para planificar tus próximas vacaciones.\n\n" +
					"📱 Teléfono: +1 (555) 019-2834\n" +
					"📧 Email: contacto@wagotviajes.com\n" +
					"🌐 Sitio Web: https://wagot.example.com\n" +
					"📸 Instagram: @wagotviajes\n\n" +
					"Encuéntranos en nuestras oficinas centrales:\n" +
					"https://maps.google.com/?q=Wagot+Viajes",
				PreviewURL: true,
			},
			FollowUpMainMenu: true,
		},
	}
}

// Flow coordinates the conversational rules.
type Flow struct {
	Greetings       []string
	MainMenu        bot.Buttons
	UnsupportedText string
	Steps           map[string]Step
}

// DefaultFlow returns the default trip agency flow configuration.
func DefaultFlow() *Flow {
	return &Flow{
		Greetings:       defaultGreetings,
		MainMenu:        MainMenuButtons(),
		UnsupportedText: "Por el momento solo puedo leer mensajes de texto.",
		Steps:           DefaultSteps(),
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
