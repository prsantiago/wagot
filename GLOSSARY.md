# wagot

A lightweight WhatsApp bot server communicating via the Meta Cloud API.

## Language

**Incoming Message**:
A message sent by a WhatsApp user to the business phone number.
_Avoid_: Inbound event, notification payload, packet

**Reply**:
An outbound message sent from the business phone number to the user in response to an incoming message.
_Avoid_: Outbound event, response payload

**Reaction**:
An emoji sent in direct reference to a specific incoming message.
_Avoid_: Emoticon, status emoji

**Conversational Handler**:
The application function that evaluates an incoming message and decides which replies or reactions to emit.
_Avoid_: Bot controller, webhook processor, message callback

**Conversation Context**:
The adapter provided to the conversational handler to inspect the incoming message and execute replies.
_Avoid_: Request context, session object
