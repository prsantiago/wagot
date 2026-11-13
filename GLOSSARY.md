# wagot

A lightweight WhatsApp bot server communicating via the Meta Cloud API.

## Language

**User**:
A person who interacts with the business over WhatsApp.
_Avoid_: Client, customer, end-user

**Incoming Message**:
A message sent by a User to the business phone number.
_Avoid_: Inbound event, notification payload, packet

**Greeting**:
An incoming message with recognized introductory keywords (such as "hola" or "menu") that initiates or resets a conversation.
_Avoid_: Salutation, handshake, wake word

**Reply**:
An outbound message sent from the business phone number to the User in response to an incoming message.
_Avoid_: Outbound event, response payload, response

**Welcome**:
An introductory text reply emitted in response to a Greeting, optionally personalized addressing the User by name.
_Avoid_: Greeting response, intro banner, hello message

**Image Reply**:
A reply that delivers an image with an optional text caption.
_Avoid_: Media payload, attachment message

**Button Reply**:
An interactive reply that presents up to three quick-action buttons below a text body.
_Avoid_: Quick reply, template button, action card

**Main Menu**:
The primary interactive button reply that presents the top-level choices guiding the User through the business offerings.
_Avoid_: Home menu, root options, navigation card, launchpad

**List Reply**:
An interactive reply that presents a menu button opening a selectable list of options.
_Avoid_: Dropdown message, picker, carousel

**Selection**:
The option chosen by a User when tapping a button or list item, identified by a stable ID and a display title.
_Avoid_: Callback data, postback payload, click event

**Step**:
The conversational action mapped to a specific selection in a flow, defining the reply emitted and whether navigation follows up with the Main Menu.
_Avoid_: Route, action handler, state transition, node

**Reaction**:
An emoji sent in direct reference to a specific incoming message.
_Avoid_: Emoticon, status emoji

**Flow**:
The deterministic conversational rules that map an incoming message to one or more replies.
_Avoid_: Script, workflow, dialog tree

**Conversational Handler**:
The application function that evaluates an incoming message and decides which replies or reactions to emit.
_Avoid_: Bot controller, webhook processor, message callback

**Conversation Context**:
The adapter provided to the conversational handler to inspect the incoming message and execute replies.
_Avoid_: Request context, session object
