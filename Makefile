# Platform CI Toolkit root Makefile.
# Delegates to the sub-project Makefiles.

.PHONY: help build-linter push-linter lint-linter

help:
	@echo "Available targets:"
	@echo "  build-linter  - Build the common linter image"
	@echo "  push-linter   - Push the common linter image"
	@echo "  lint-linter   - Run the common linter against this repository"

build-linter:
	$(MAKE) -C linter build

push-linter:
	$(MAKE) -C linter push

lint-linter:
	$(MAKE) -C linter lint
