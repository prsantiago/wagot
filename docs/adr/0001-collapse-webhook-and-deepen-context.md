# 0001: Encapsulate Webhook Ingress and Deepen Conversational Handler

We decided to collapse the shallow `internal/webhook` package directly into `internal/bot` and introduce a high-level `bot.Context` conversational adapter with an unnested `bot.Message` struct.

`internal/webhook` previously leaked 4-level nested Meta wire DTOs and forced callers in `main.go` to import three packages (`webhook`, `bot`, and `cloudapi`) and traverse raw pointer structs.

By having `*bot.Bot` implement standard `http.Handler` directly, all Meta verification handshakes, HMAC-SHA256 signature checks, and wire JSON unmarshaling remain private to `internal/bot`. The conversational handler receives `*bot.Context` (`func(c *bot.Context) error`) with a flat `c.Message` value struct, providing direct intent methods (`c.Reply(text)` and `c.React(emoji)`) that dispatch immediately over HTTP.

