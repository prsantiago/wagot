# 0002: Declarative Replies on Context.Send and Data-Driven Flow Engine

We decided to extend `*bot.Context` with a single `c.Send(r bot.Reply) error` method backed by self-describing value structs (`bot.Text`, `bot.Image`, `bot.Buttons`, `bot.List`), and to drive business logic through a declarative `internal/flow` package configured via JSON rather than hardcoded `switch` handlers.

Adding dedicated positional methods (`ReplyImage`, `ReplyButtons`, `ReplyList`) for every WhatsApp message shape widens the `bot.Context` interface, creates ambiguous positional string arguments at call sites, and breaks signatures whenever optional fields (such as headers, footers, or URL previews) are introduced. Similarly, hardcoding a specific business's menu tree in Go forces code edits and recompilation to onboard another agency or business vertical.

By representing replies as pure data values dispatched through `c.Send` (while preserving `c.Reply(text)` and `c.React(emoji)` as convenience shorthands) and exposing `Message.SelectionID` for incoming button and list interactions, `internal/flow` can map greeting keywords and selection IDs directly to declarative steps loaded from a JSON file (with an embedded default). Onboarding a new business requires only swapping the JSON configuration file.
