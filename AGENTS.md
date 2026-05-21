# Agents

This implementation is inspired by the [OpenAPIGenerator](https://github.com/OpenAPITools/openapi-generator)

## Development

- Don't edit generated client code directly, instead edit the templates at `templates/*` or logic at `internal/generators/*`
- Don't add `Co-authored-by: Copilot` to commit messages
- Before generating code delete the `generated/` directory to avoid stale files
