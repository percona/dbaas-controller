# Catalog

This package holds i18n for DBaaS Controller.

[gotext](https://pkg.go.dev/golang.org/x/text@v0.3.3/cmd/gotext?tab=doc)

To translate, we need to follow the next steps.

- Run `make gen` to collect all messages from source code; it also merges them with existing translation in `messages.gotext.json` and writes them into `out.gotext.json`.
- As a result, we will receive all new (not translated messages) and all old (translated messages from `messages.gotext.json`) in `out.gotext.json`.
- In the next step, we need to copy `out.gotext.json` into messages.gotext.json and fill in all required translation values.
- When we run `make gen` again, it generate `catalog.go` with all-new translation.
- Now after 'make release', new translated messages will be used in a binary file.

## Files

- `messages.gotext.json` contains translated messages.
- `out.gotext.json` is intermidiate file; contains merged entities from `messages.gotext.json` and parsed messages from soure code.
- `catalog.go` generated file; includes content of `messages.gotext.json`. BDaaS-controller use it by to choose right translation.
