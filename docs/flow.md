# Flow Configuration

Wagot allows operators to configure deterministic conversational rules via an external JSON file specified through the `-flow-file` CLI flag or `FLOW_FILE` environment variable.

A complete reference template is provided at [flow.example.json](../flow.example.json).

## Schema

### Top-Level Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `greetings` | `[]string` | Optional | Incoming keywords (case-insensitive, normalized for punctuation) that trigger the welcome greeting. Defaults to `["hola", "informacion", "menu"]`. |
| `welcome_template` | `string` | Optional | Welcome message template addressing the User by name using a `{{name}}` placeholder (e.g. `"¡Hola, {{name}}! Bienvenido"`). |
| `welcome_fallback` | `string` | Optional | Welcome message sent if the User's WhatsApp profile name is not present. |
| `unsupported_text` | `string` | Optional | Text reply sent when the User sends an unsupported message type (audio, video, sticker, location). |
| `main_menu` | `object` | **Required** | The default Button Reply presented after greetings or when no specific step matches. |
| `steps` | `object` | Optional | Map of Selection IDs (matching button/list IDs) to step definitions. |

### Main Menu (`main_menu`)

The Main Menu is an interactive Button Reply:

```json
"main_menu": {
  "body": "¿En qué podemos ayudarte hoy?",
  "buttons": [
    {"id": "trips", "title": "Nuestros viajes"},
    {"id": "contact", "title": "Contacto"}
  ]
}
```

- `body`: Text displayed above the buttons (required).
- `buttons`: 1 to 3 buttons, each with `id` and `title` (required).

### Step Definitions (`steps`)

Each key in `steps` corresponds to a `selection_id` sent when a User taps an interactive button or list item.

```json
"steps": {
  "<selection_id>": {
    "reply": {
      "type": "text | image | buttons | list",
      ...
    },
    "follow_up_main_menu": false
  }
}
```

- `follow_up_main_menu` (`bool`): When `true`, wagot immediately sends the Main Menu Button Reply after this step's reply so the conversation does not dead-end.

#### Reply Types

1. **Text Reply (`"type": "text"`):**
   - `body` (`string`, required): Text message content.
   - `preview_url` (`bool`, optional): Whether to enable URL preview for web links contained in `body`.

2. **Image Reply (`"type": "image"`):**
   - `url` (`string`, required): Public HTTPS image URL accessible by Meta's Cloud API servers.
   - `caption` (`string`, optional): Descriptive caption displayed below the image.

3. **Button Reply (`"type": "buttons"`):**
   - `body` (`string`, required): Text message content.
   - `buttons` (`[]object`, required): 1 to 3 buttons with `id` and `title`.

4. **List Reply (`"type": "list"`):**
   - `body` (`string`, required): Text message content.
   - `button` (`string`, required): Drawer menu button label (e.g. `"Ver opciones"`).
   - `items` (`[]object`, required): 1 to 10 list items, each with `id` (`string`), `title` (`string`), and optional `description` (`string`).
