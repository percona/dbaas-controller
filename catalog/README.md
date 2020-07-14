# Catalog

This package holds i18n for dbaas-controller.

[gotext](https://pkg.go.dev/golang.org/x/text@v0.3.3/cmd/gotext?tab=doc)

To translate dbaas-controller's messages, we need to follow the next steps.

- Use shared `golang.org/x/text/message.Printer` object's `Sprintf` method for messages that should be translated.
  Do not use `Sprint` as it will not be collected on the next step.
- Run `make gen` to collect new messages from source code and merge them with existing translations in `messages.gotext.json`.
- The output of the command above will contain messages starting with "en: Missing entry for XXX".
  That's new messages that should be translated in that file. Fill `translation` fields.
- Run `make gen` to regenerate `catalog.go` file.
- Compile Go program as usual (`make release`) to use updated translations.

## Files

- `messages.gotext.json` contains translated messages.
- `catalog.go` generated file; includes the content of `messages.gotext.json`.
  dbaas-controller uses it to choose the right translation.
