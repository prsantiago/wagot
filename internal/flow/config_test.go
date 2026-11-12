package flow_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chachin/wagot/internal/bot"
	"github.com/chachin/wagot/internal/cloudapi"
	"github.com/chachin/wagot/internal/flow"
)

type recordSender struct {
	msgs []cloudapi.Message
}

func (s *recordSender) Send(ctx context.Context, phoneNumberID, to string, msg cloudapi.Message) (*cloudapi.SendResponse, error) {
	s.msgs = append(s.msgs, msg)
	return &cloudapi.SendResponse{}, nil
}

func (s *recordSender) MarkRead(ctx context.Context, phoneNumberID, messageID string, typing bool) error {
	return nil
}

func TestLoadCustomFlowFileGreetingsAndMainMenu(t *testing.T) {
	jsonContent := `{
		"greetings": ["hola", "pedir"],
		"welcome_template": "¡Hola, %s! Te damos la bienvenida a La Pizzería 🍕",
		"welcome_fallback": "¡Hola! Te damos la bienvenida a La Pizzería 🍕",
		"main_menu": {
			"body": "¿Qué te gustaría ordenar hoy?",
			"buttons": [
				{"id": "pizzas", "title": "Ver pizzas"},
				{"id": "promos", "title": "Promociones"}
			]
		}
	}`

	tmpDir := t.TempDir()
	flowPath := filepath.Join(tmpDir, "flow.json")
	if err := os.WriteFile(flowPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fl, err := flow.LoadFile(flowPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	sender := &recordSender{}
	msg := bot.Message{
		ID:   "wamid.custom_greet",
		From: "15551234567",
		Type: bot.MessageTypeText,
		Text: "pedir",
	}
	c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)
	c.Contact = bot.Contact{
		WaID:    "15551234567",
		Profile: bot.Profile{Name: "Santiago"},
	}

	if err := fl.Handle(c); err != nil {
		t.Fatalf("fl.Handle error: %v", err)
	}

	if len(sender.msgs) != 2 {
		t.Fatalf("got %d messages, want 2 (welcome + main menu)", len(sender.msgs))
	}

	// 1. Welcome text addressing Santiago with custom pizzeria copy
	m1 := sender.msgs[0]
	if m1.Type != "text" || m1.Text == nil {
		t.Fatalf("m1 = %+v, want text", m1)
	}
	if !strings.Contains(m1.Text.Body, "Santiago") || !strings.Contains(m1.Text.Body, "La Pizzería") {
		t.Errorf("m1.Text.Body = %q, want greeting mentioning Santiago and La Pizzería", m1.Text.Body)
	}

	// 2. Main menu button reply with custom buttons
	m2 := sender.msgs[1]
	if m2.Type != "interactive" || m2.Interactive == nil || m2.Interactive.Type != "button" {
		t.Fatalf("m2 = %+v, want button interactive", m2)
	}
	if m2.Interactive.Body.Text != "¿Qué te gustaría ordenar hoy?" {
		t.Errorf("m2 body = %q", m2.Interactive.Body.Text)
	}
	btns := m2.Interactive.Action.Buttons
	if len(btns) != 2 {
		t.Fatalf("btns = %+v, want 2", btns)
	}
	if btns[0].Reply.ID != "pizzas" || btns[0].Reply.Title != "Ver pizzas" {
		t.Errorf("btn[0] = %+v", btns[0])
	}
	if btns[1].Reply.ID != "promos" || btns[1].Reply.Title != "Promociones" {
		t.Errorf("btn[1] = %+v", btns[1])
	}
}

func TestLoadFlowValidationErrors(t *testing.T) {
	tests := map[string]struct {
		json    string
		wantErr string
	}{
		"malformed json": {
			json:    `{ invalid json `,
			wantErr: "unmarshal flow json",
		},
		"missing main menu body": {
			json: `{
				"main_menu": {
					"body": "",
					"buttons": [{"id": "b1", "title": "B1"}]
				}
			}`,
			wantErr: "main_menu body is required",
		},
		"empty main menu buttons": {
			json: `{
				"main_menu": {
					"body": "Select an option",
					"buttons": []
				}
			}`,
			wantErr: "main_menu must have between 1 and 3 buttons",
		},
		"more than 3 main menu buttons": {
			json: `{
				"main_menu": {
					"body": "Select an option",
					"buttons": [
						{"id": "b1", "title": "B1"},
						{"id": "b2", "title": "B2"},
						{"id": "b3", "title": "B3"},
						{"id": "b4", "title": "B4"}
					]
				}
			}`,
			wantErr: "main_menu must have between 1 and 3 buttons",
		},
		"main menu button missing id": {
			json: `{
				"main_menu": {
					"body": "Select an option",
					"buttons": [{"id": "", "title": "B1"}]
				}
			}`,
			wantErr: "button id cannot be empty",
		},
		"main menu button missing title": {
			json: `{
				"main_menu": {
					"body": "Select an option",
					"buttons": [{"id": "b1", "title": ""}]
				}
			}`,
			wantErr: "button title cannot be empty",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := flow.LoadJSON([]byte(tc.json))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("LoadJSON() err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestDefaultFlowValidates(t *testing.T) {
	fl := flow.DefaultFlow()
	if err := fl.Validate(); err != nil {
		t.Fatalf("DefaultFlow().Validate() error: %v", err)
	}
}

func TestFlowExampleJSONValidAndMatchesDefault(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	examplePath := filepath.Join(repoRoot, "flow.example.json")

	fl, err := flow.LoadFile(examplePath)
	if err != nil {
		t.Fatalf("LoadFile(%q) error: %v", examplePath, err)
	}

	if err := fl.Validate(); err != nil {
		t.Fatalf("Validate() on example flow error: %v", err)
	}

	def := flow.DefaultFlow()

	if len(fl.Greetings) != len(def.Greetings) {
		t.Errorf("greetings count = %d, want %d", len(fl.Greetings), len(def.Greetings))
	}
	if fl.MainMenu.Body != def.MainMenu.Body {
		t.Errorf("main_menu body = %q, want %q", fl.MainMenu.Body, def.MainMenu.Body)
	}
	if len(fl.MainMenu.Buttons) != len(def.MainMenu.Buttons) {
		t.Fatalf("main_menu buttons = %d, want %d", len(fl.MainMenu.Buttons), len(def.MainMenu.Buttons))
	}
	for i := range def.MainMenu.Buttons {
		if fl.MainMenu.Buttons[i] != def.MainMenu.Buttons[i] {
			t.Errorf("button[%d] = %+v, want %+v", i, fl.MainMenu.Buttons[i], def.MainMenu.Buttons[i])
		}
	}

	if len(fl.Steps) != len(def.Steps) {
		t.Errorf("steps count = %d, want %d", len(fl.Steps), len(def.Steps))
	}
	for id, defStep := range def.Steps {
		exStep, ok := fl.Steps[id]
		if !ok {
			t.Errorf("missing step %q in flow.example.json", id)
			continue
		}
		if exStep.FollowUpMainMenu != defStep.FollowUpMainMenu {
			t.Errorf("step %q FollowUpMainMenu = %v, want %v", id, exStep.FollowUpMainMenu, defStep.FollowUpMainMenu)
		}
		if !reflect.DeepEqual(exStep.Reply, defStep.Reply) {
			t.Errorf("step %q reply = %+v, want %+v", id, exStep.Reply, defStep.Reply)
		}
	}
}

func TestLoadFlowFileNotFound(t *testing.T) {
	_, err := flow.LoadFile("non_existent_flow_file_12345.json")
	if err == nil {
		t.Fatal("LoadFile succeeded for non-existent file, want error")
	}
}

func TestInteractiveStepsLoadedFromJSON(t *testing.T) {
	jsonContent := `{
		"greetings": ["hola"],
		"main_menu": {
			"body": "Menú principal",
			"buttons": [
				{"id": "menu_list", "title": "Lista de platos"},
				{"id": "contact_info", "title": "Contacto"}
			]
		},
		"steps": {
			"menu_list": {
				"reply": {
					"type": "list",
					"body": "Nuestra carta gastronómica:",
					"button": "Ver platos",
					"items": [
						{"id": "item_pizza", "title": "Pizza Artesanal", "description": "Horno a la leña"},
						{"id": "item_pasta", "title": "Pasta Fresca", "description": "Salsa pomodoro"}
					]
				},
				"follow_up_main_menu": false
			},
			"item_pizza": {
				"reply": {
					"type": "image",
					"url": "https://images.example.com/pizza.jpg",
					"caption": "Deliciosa pizza margarita."
				},
				"follow_up_main_menu": true
			},
			"contact_info": {
				"reply": {
					"type": "text",
					"body": "Visítanos en https://maps.google.com/?q=Pizzeria",
					"preview_url": true
				},
				"follow_up_main_menu": true
			},
			"quick_feedback": {
				"reply": {
					"type": "buttons",
					"body": "¿Cómo calificarías tu experiencia?",
					"buttons": [
						{"id": "rate_good", "title": "Buena"},
						{"id": "rate_bad", "title": "Mala"}
					]
				},
				"follow_up_main_menu": false
			}
		}
	}`

	fl, err := flow.LoadJSON([]byte(jsonContent))
	if err != nil {
		t.Fatalf("LoadJSON() err = %v", err)
	}

	// 1. List reply test
	t.Run("list reply step", func(t *testing.T) {
		sender := &recordSender{}
		msg := bot.Message{
			ID:          "wamid.step1",
			From:        "15551234567",
			Type:        bot.MessageTypeInteractive,
			SelectionID: "menu_list",
		}
		c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)

		if err := fl.Handle(c); err != nil {
			t.Fatalf("fl.Handle: %v", err)
		}

		if len(sender.msgs) != 1 {
			t.Fatalf("got %d messages, want 1", len(sender.msgs))
		}
		sent := sender.msgs[0]
		if sent.Type != "interactive" || sent.Interactive == nil || sent.Interactive.Type != "list" {
			t.Fatalf("sent = %+v, want list reply", sent)
		}
		if sent.Interactive.Action.Button != "Ver platos" {
			t.Errorf("button label = %q, want 'Ver platos'", sent.Interactive.Action.Button)
		}
		if len(sent.Interactive.Action.Sections[0].Rows) != 2 {
			t.Errorf("rows = %d, want 2", len(sent.Interactive.Action.Sections[0].Rows))
		}
	})

	// 2. Image reply with follow up main menu
	t.Run("image reply with follow-up main menu", func(t *testing.T) {
		sender := &recordSender{}
		msg := bot.Message{
			ID:          "wamid.step2",
			From:        "15551234567",
			Type:        bot.MessageTypeInteractive,
			SelectionID: "item_pizza",
		}
		c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)

		if err := fl.Handle(c); err != nil {
			t.Fatalf("fl.Handle: %v", err)
		}

		if len(sender.msgs) != 2 {
			t.Fatalf("got %d messages, want 2 (image + main menu)", len(sender.msgs))
		}
		if sender.msgs[0].Type != "image" || sender.msgs[0].Image == nil {
			t.Fatalf("msg[0] = %+v, want image", sender.msgs[0])
		}
		if sender.msgs[0].Image.Link != "https://images.example.com/pizza.jpg" {
			t.Errorf("image link = %q", sender.msgs[0].Image.Link)
		}
		if sender.msgs[0].Image.Caption != "Deliciosa pizza margarita." {
			t.Errorf("caption = %q", sender.msgs[0].Image.Caption)
		}
		// Second message must be main menu
		if sender.msgs[1].Type != "interactive" || sender.msgs[1].Interactive.Type != "button" {
			t.Errorf("msg[1] = %+v, want main menu button reply", sender.msgs[1])
		}
	})

	// 3. Text reply with preview_url and follow up main menu
	t.Run("text reply with preview and follow-up main menu", func(t *testing.T) {
		sender := &recordSender{}
		msg := bot.Message{
			ID:          "wamid.step3",
			From:        "15551234567",
			Type:        bot.MessageTypeInteractive,
			SelectionID: "contact_info",
		}
		c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)

		if err := fl.Handle(c); err != nil {
			t.Fatalf("fl.Handle: %v", err)
		}

		if len(sender.msgs) != 2 {
			t.Fatalf("got %d messages, want 2 (text + main menu)", len(sender.msgs))
		}
		if sender.msgs[0].Type != "text" || sender.msgs[0].Text == nil {
			t.Fatalf("msg[0] = %+v, want text", sender.msgs[0])
		}
		if !sender.msgs[0].Text.PreviewURL {
			t.Errorf("PreviewURL = false, want true")
		}
		if sender.msgs[1].Type != "interactive" || sender.msgs[1].Interactive.Type != "button" {
			t.Errorf("msg[1] = %+v, want main menu button reply", sender.msgs[1])
		}
	})

	// 4. Button reply step
	t.Run("button reply step", func(t *testing.T) {
		sender := &recordSender{}
		msg := bot.Message{
			ID:          "wamid.step4",
			From:        "15551234567",
			Type:        bot.MessageTypeInteractive,
			SelectionID: "quick_feedback",
		}
		c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)

		if err := fl.Handle(c); err != nil {
			t.Fatalf("fl.Handle: %v", err)
		}

		if len(sender.msgs) != 1 {
			t.Fatalf("got %d messages, want 1", len(sender.msgs))
		}
		if sender.msgs[0].Type != "interactive" || sender.msgs[0].Interactive.Type != "button" {
			t.Fatalf("msg[0] = %+v, want button reply", sender.msgs[0])
		}
		if sender.msgs[0].Interactive.Body.Text != "¿Cómo calificarías tu experiencia?" {
			t.Errorf("body = %q", sender.msgs[0].Interactive.Body.Text)
		}
	})
}

func TestStepReplyValidationErrors(t *testing.T) {
	tests := map[string]struct {
		json    string
		wantErr string
	}{
		"unknown step reply type": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {"reply": {"type": "audio", "url": "https://example.com/audio.mp3"}}
				}
			}`,
			wantErr: "unknown step type",
		},
		"empty text reply body": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {"reply": {"type": "text", "body": "   "}}
				}
			}`,
			wantErr: "text reply body cannot be empty",
		},
		"empty image reply url": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {"reply": {"type": "image", "url": "", "caption": "Photo"}}
				}
			}`,
			wantErr: "image reply url cannot be empty",
		},
		"non-https image reply url": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {"reply": {"type": "image", "url": "http://insecure.example.com/photo.jpg"}}
				}
			}`,
			wantErr: "image reply url must be a public HTTPS URL",
		},
		"button reply step empty body": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {
						"reply": {
							"type": "buttons",
							"body": "",
							"buttons": [{"id": "b1", "title": "B1"}]
						}
					}
				}
			}`,
			wantErr: "buttons reply body is required",
		},
		"button reply step empty buttons": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {
						"reply": {
							"type": "buttons",
							"body": "Pick one",
							"buttons": []
						}
					}
				}
			}`,
			wantErr: "buttons reply must have between 1 and 3 buttons",
		},
		"button reply step more than 3 buttons": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {
						"reply": {
							"type": "buttons",
							"body": "Pick one",
							"buttons": [
								{"id": "b1", "title": "B1"},
								{"id": "b2", "title": "B2"},
								{"id": "b3", "title": "B3"},
								{"id": "b4", "title": "B4"}
							]
						}
					}
				}
			}`,
			wantErr: "buttons reply must have between 1 and 3 buttons",
		},
		"button reply step button missing id": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {
						"reply": {
							"type": "buttons",
							"body": "Pick one",
							"buttons": [{"id": "", "title": "B1"}]
						}
					}
				}
			}`,
			wantErr: "button id cannot be empty",
		},
		"button reply step button missing title": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {
						"reply": {
							"type": "buttons",
							"body": "Pick one",
							"buttons": [{"id": "b1", "title": ""}]
						}
					}
				}
			}`,
			wantErr: "button title cannot be empty",
		},
		"list reply step empty body": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {
						"reply": {
							"type": "list",
							"body": "",
							"button": "View",
							"items": [{"id": "i1", "title": "T1"}]
						}
					}
				}
			}`,
			wantErr: "list reply body cannot be empty",
		},
		"list reply step empty button label": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {
						"reply": {
							"type": "list",
							"body": "Options",
							"button": "   ",
							"items": [{"id": "i1", "title": "T1"}]
						}
					}
				}
			}`,
			wantErr: "list reply button label cannot be empty",
		},
		"list reply step empty items": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {
						"reply": {
							"type": "list",
							"body": "Options",
							"button": "View",
							"items": []
						}
					}
				}
			}`,
			wantErr: "list reply must have between 1 and 10 items",
		},
		"list reply step item missing id": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {
						"reply": {
							"type": "list",
							"body": "Options",
							"button": "View",
							"items": [{"id": "", "title": "T1"}]
						}
					}
				}
			}`,
			wantErr: "list item id cannot be empty",
		},
		"list reply step item missing title": {
			json: `{
				"main_menu": {"body": "Menu", "buttons": [{"id": "b1", "title": "B1"}]},
				"steps": {
					"step1": {
						"reply": {
							"type": "list",
							"body": "Options",
							"button": "View",
							"items": [{"id": "i1", "title": ""}]
						}
					}
				}
			}`,
			wantErr: "list item title cannot be empty",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := flow.LoadJSON([]byte(tc.json))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("LoadJSON() err = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestWelcomeCopySingleFieldAndFallback(t *testing.T) {
	t.Run("single welcome field works with and without profile name", func(t *testing.T) {
		jsonContent := `{
			"welcome": "¡Bienvenidos a Dulces & Pasteles!",
			"main_menu": {
				"body": "¿Qué postre deseas?",
				"buttons": [{"id": "cakes", "title": "Pasteles"}]
			}
		}`
		fl, err := flow.LoadJSON([]byte(jsonContent))
		if err != nil {
			t.Fatalf("LoadJSON() err = %v", err)
		}

		sender := &recordSender{}
		msg := bot.Message{
			ID:   "wamid.greet",
			From: "15551234567",
			Type: bot.MessageTypeText,
			Text: "hola",
		}
		c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)
		c.Contact = bot.Contact{WaID: "15551234567", Profile: bot.Profile{Name: ""}}

		if err := fl.Handle(c); err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if len(sender.msgs) != 2 {
			t.Fatalf("got %d messages, want 2", len(sender.msgs))
		}
		if sender.msgs[0].Text.Body != "¡Bienvenidos a Dulces & Pasteles!" {
			t.Errorf("welcome body = %q", sender.msgs[0].Text.Body)
		}
	})

	t.Run("welcome template falls back cleanly when user profile name is empty", func(t *testing.T) {
		jsonContent := `{
			"welcome_template": "¡Hola, %s! Te damos la bienvenida.",
			"main_menu": {
				"body": "Menú",
				"buttons": [{"id": "b1", "title": "B1"}]
			}
		}`
		fl, err := flow.LoadJSON([]byte(jsonContent))
		if err != nil {
			t.Fatalf("LoadJSON() err = %v", err)
		}

		sender := &recordSender{}
		msg := bot.Message{
			ID:   "wamid.greet",
			From: "15551234567",
			Type: bot.MessageTypeText,
			Text: "hola",
		}
		c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)
		c.Contact = bot.Contact{WaID: "15551234567", Profile: bot.Profile{Name: ""}}

		if err := fl.Handle(c); err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if len(sender.msgs) != 2 {
			t.Fatalf("got %d messages, want 2", len(sender.msgs))
		}
		if !strings.Contains(sender.msgs[0].Text.Body, "bienvenida") {
			t.Errorf("welcome body = %q, want non-empty fallback welcome", sender.msgs[0].Text.Body)
		}
	})
}

func TestGreetingKeywordsWithAccentsAndCasing(t *testing.T) {
	jsonContent := `{
		"greetings": ["Hola", "Menú", "Información"],
		"welcome": "¡Bienvenido a nuestra tienda!",
		"main_menu": {
			"body": "Menú",
			"buttons": [{"id": "b1", "title": "B1"}]
		}
	}`
	fl, err := flow.LoadJSON([]byte(jsonContent))
	if err != nil {
		t.Fatalf("LoadJSON() err = %v", err)
	}

	testInputs := []string{"hola", "HOLA", "¡Hola!", "menu", "¡Menú!", "informacion", "¿Información?"}
	for _, in := range testInputs {
		sender := &recordSender{}
		msg := bot.Message{
			ID:   "wamid.kw",
			From: "15551234567",
			Type: bot.MessageTypeText,
			Text: in,
		}
		c := bot.NewContext(context.Background(), sender, "PNID_TEST", msg)
		if err := fl.Handle(c); err != nil {
			t.Fatalf("Handle(%q) err = %v", in, err)
		}
		if len(sender.msgs) != 2 {
			t.Errorf("input %q got %d messages, want 2 (greeting recognized)", in, len(sender.msgs))
		}
	}
}
