# Catalog

This package holds i18n for dbaas-controller.

[gotext](https://pkg.go.dev/golang.org/x/text@v0.3.3/cmd/gotext?tab=doc)

To translate dbaas-controller's messages, we need to follow the next steps.

- Run `make gen` to collect all messages from source code; it also merges them with existing translation in `messages.gotext.json` and writes them into `out.gotext.json`.
- As a result, we will receive all new (not translated messages) and all old (translated messages from `messages.gotext.json`) in `out.gotext.json`.
- In the next step, it replace `messages.gotext.json` with new `out.gotext.json`.
- Fill in all required translation values in `messages.gotext.json`.
- When we run `make gen` again, it generate `catalog.go` with all-new translation.
- Now after 'make release', new translated messages will be used in a binary file.

## Files

- `messages.gotext.json` contains translated messages.
- `catalog.go` generated file; includes the content of `messages.gotext.json`. dbaas-controller uses it to choose the right translation.
