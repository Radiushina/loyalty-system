.PHONY: api-build
api-build:
	docker run --rm -v ${PWD}/docs:/spec redocly/cli build-docs --config redocly.yml -o api.html openapi.yml