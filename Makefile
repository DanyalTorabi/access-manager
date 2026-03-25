# Delegate to the Go implementation under go/ (T29). Override: `make -C go test`.
.PHONY: build test cover cover-func run tidy lint
build test cover cover-func run tidy lint:
	$(MAKE) -C go $@
