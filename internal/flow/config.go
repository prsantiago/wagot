package flow

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/chachin/wagot/internal/bot"
)

type rawFlow struct {
	Greetings       []string           `json:"greetings"`
	Welcome         string             `json:"welcome"`
	WelcomeTemplate string             `json:"welcome_template"`
	WelcomeFallback string             `json:"welcome_fallback"`
	UnsupportedText string             `json:"unsupported_text"`
	MainMenu        rawButtons         `json:"main_menu"`
	Steps           map[string]rawStep `json:"steps"`
}

type rawButtons struct {
	Body    string       `json:"body"`
	Buttons []bot.Button `json:"buttons"`
}

type rawStep struct {
	Reply            rawReply `json:"reply"`
	FollowUpMainMenu bool     `json:"follow_up_main_menu"`
}

type rawReply struct {
	Type       string         `json:"type"`
	Body       string         `json:"body"`
	Button     string         `json:"button"`
	Buttons    []bot.Button   `json:"buttons"`
	Items      []bot.ListItem `json:"items"`
	URL        string         `json:"url"`
	Caption    string         `json:"caption"`
	PreviewURL bool           `json:"preview_url"`
}

func parseStep(raw rawStep) (Step, error) {
	step := Step{
		FollowUpMainMenu: raw.FollowUpMainMenu,
	}

	switch strings.ToLower(raw.Reply.Type) {
	case "text":
		step.Reply = bot.Text{
			Body:       raw.Reply.Body,
			PreviewURL: raw.Reply.PreviewURL,
		}
	case "image":
		step.Reply = bot.Image{
			URL:     raw.Reply.URL,
			Caption: raw.Reply.Caption,
		}
	case "button", "buttons":
		step.Reply = bot.Buttons{
			Body:    raw.Reply.Body,
			Buttons: raw.Reply.Buttons,
		}
	case "list":
		step.Reply = bot.List{
			Body:   raw.Reply.Body,
			Button: raw.Reply.Button,
			Items:  raw.Reply.Items,
		}
	default:
		return Step{}, fmt.Errorf("unknown step type %q", raw.Reply.Type)
	}

	return step, nil
}

// LoadFile reads a JSON flow configuration from path and validates it.
func LoadFile(path string) (*Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read flow file: %w", err)
	}
	return LoadJSON(data)
}

// LoadJSON parses and validates a JSON flow configuration.
func LoadJSON(data []byte) (*Flow, error) {
	var raw rawFlow
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal flow json: %w", err)
	}

	steps := make(map[string]Step, len(raw.Steps))
	for id, s := range raw.Steps {
		parsed, err := parseStep(s)
		if err != nil {
			return nil, fmt.Errorf("step %q: %w", id, err)
		}
		steps[id] = parsed
	}

	welcomeTemplate := raw.WelcomeTemplate
	welcomeFallback := raw.WelcomeFallback
	if raw.Welcome != "" {
		if welcomeTemplate == "" {
			welcomeTemplate = raw.Welcome
		}
		if welcomeFallback == "" {
			welcomeFallback = raw.Welcome
		}
	}

	greetings := raw.Greetings
	if len(greetings) == 0 {
		greetings = []string{"hola", "informacion", "menu"}
	}

	fl := &Flow{
		Greetings:       greetings,
		WelcomeTemplate: welcomeTemplate,
		WelcomeFallback: welcomeFallback,
		UnsupportedText: raw.UnsupportedText,
		MainMenu: bot.Buttons{
			Body:    raw.MainMenu.Body,
			Buttons: raw.MainMenu.Buttons,
		},
		Steps: steps,
	}

	if err := fl.Validate(); err != nil {
		return nil, fmt.Errorf("invalid flow configuration: %w", err)
	}
	return fl, nil
}

// Validate checks that the flow configuration adheres to all required constraints.
func (f *Flow) Validate() error {
	var errs []error
	errs = append(errs, validateButtons("main_menu", f.MainMenu)...)
	for id, s := range f.Steps {
		if err := validateReply(id, s.Reply); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func validateButtons(prefix string, b bot.Buttons) []error {
	var errs []error
	if strings.TrimSpace(b.Body) == "" {
		errs = append(errs, fmt.Errorf("%s body is required", prefix))
	}
	if len(b.Buttons) < 1 || len(b.Buttons) > 3 {
		errs = append(errs, fmt.Errorf("%s must have between 1 and 3 buttons", prefix))
	}
	for i, btn := range b.Buttons {
		if strings.TrimSpace(btn.ID) == "" {
			errs = append(errs, fmt.Errorf("%s button[%d]: button id cannot be empty", prefix, i))
		}
		if strings.TrimSpace(btn.Title) == "" {
			errs = append(errs, fmt.Errorf("%s button[%d]: button title cannot be empty", prefix, i))
		}
	}
	return errs
}

func validateReply(stepID string, r bot.Reply) error {
	if r == nil {
		return fmt.Errorf("step %q: reply cannot be nil", stepID)
	}
	var errs []error
	switch rep := r.(type) {
	case bot.Text:
		if strings.TrimSpace(rep.Body) == "" {
			errs = append(errs, fmt.Errorf("step %q: text reply body cannot be empty", stepID))
		}
	case bot.Image:
		if strings.TrimSpace(rep.URL) == "" {
			errs = append(errs, fmt.Errorf("step %q: image reply url cannot be empty", stepID))
		} else if !strings.HasPrefix(strings.ToLower(rep.URL), "https://") {
			errs = append(errs, fmt.Errorf("step %q: image reply url must be a public HTTPS URL, got %q", stepID, rep.URL))
		}
	case bot.Buttons:
		errs = append(errs, validateButtons(fmt.Sprintf("step %q: buttons reply", stepID), rep)...)
	case bot.List:
		if strings.TrimSpace(rep.Body) == "" {
			errs = append(errs, fmt.Errorf("step %q: list reply body cannot be empty", stepID))
		}
		if strings.TrimSpace(rep.Button) == "" {
			errs = append(errs, fmt.Errorf("step %q: list reply button label cannot be empty", stepID))
		}
		if len(rep.Items) < 1 || len(rep.Items) > 10 {
			errs = append(errs, fmt.Errorf("step %q: list reply must have between 1 and 10 items", stepID))
		}
		for i, it := range rep.Items {
			if strings.TrimSpace(it.ID) == "" {
				errs = append(errs, fmt.Errorf("step %q: item[%d]: list item id cannot be empty", stepID, i))
			}
			if strings.TrimSpace(it.Title) == "" {
				errs = append(errs, fmt.Errorf("step %q: item[%d]: list item title cannot be empty", stepID, i))
			}
		}
	default:
		errs = append(errs, fmt.Errorf("step %q: unknown step reply type %T", stepID, r))
	}
	return errors.Join(errs...)
}
