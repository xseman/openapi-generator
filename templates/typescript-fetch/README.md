# TypeScript Fetch Generator

Generates a TypeScript client library using the native Fetch API. This generator
produces clean, type-safe code compatible with modern TypeScript projects.

## Features

- Native Fetch API (no external HTTP client dependencies)
- Full TypeScript type safety with generated interfaces
- Support for OpenAPI 3.x and Swagger 2.0 specifications
- Runtime validation with FromJSON/ToJSON converters
- Enum support (string enums or const objects)
- Configurable file naming conventions
- Optional NPM package scaffolding

## Usage

### Basic Generation

```bash
openapi-generator generate \
  -i openapi.yaml \
  -g typescript-fetch \
  -o ./generated
```

### With NPM Package

Generate a complete NPM package with package.json and tsconfig.json:

```bash
openapi-generator generate \
  -i openapi.yaml \
  -g typescript-fetch \
  -o ./generated \
  -p withPackageJson=true \
  -p npmName=@company/api-client \
  -p npmVersion=1.0.0
```

### With TypeScript Interfaces

Generate interfaces alongside classes for better type flexibility:

```bash
openapi-generator generate \
  -i openapi.yaml \
  -g typescript-fetch \
  -o ./generated \
  -p withInterfaces=true
```

### Production Configuration

Recommended settings for production use:

```bash
openapi-generator generate \
  -i openapi.yaml \
  -g typescript-fetch \
  -o ./src/generated \
  -p withPackageJson=true \
  -p withInterfaces=true \
  -p stringEnums=true \
  -p fileNaming=kebab-case \
  -p useSingleRequestParameter=true \
  -p prefixParameterInterfaces=true
```

## Configuration Options

| Property                    | Type    | Default   | Description                                           |
| --------------------------- | ------- | --------- | ----------------------------------------------------- |
| `withPackageJson`           | boolean | false     | Generate package.json and tsconfig.json files         |
| `withInterfaces`            | boolean | false     | Generate interfaces alongside classes                 |
| `useSingleRequestParameter` | boolean | true      | Use single request object for method parameters       |
| `prefixParameterInterfaces` | boolean | false     | Prefix parameter interfaces with API class name       |
| `withoutRuntimeChecks`      | boolean | false     | Skip runtime type validation (FromJSON/ToJSON)        |
| `stringEnums`               | boolean | false     | Generate string enums instead of const objects        |
| `importFileExtension`       | string  | ""        | File extension for imports (e.g., `.js` for ESM)      |
| `fileNaming`                | string  | camelCase | File naming convention (PascalCase, camelCase, kebab-case) |
| `validationAttributes`      | boolean | false     | Generate validation metadata for properties           |
| `npmName`                   | string  | -         | NPM package name (when withPackageJson=true)          |
| `npmVersion`                | string  | 1.0.0     | NPM package version (when withPackageJson=true)       |
| `npmRepository`             | string  | -         | NPM registry URL (when withPackageJson=true)          |

### Configuration File

You can also use a configuration file instead of command-line flags:

```yaml
# config.yaml
withPackageJson: true
withInterfaces: true
stringEnums: true
fileNaming: kebab-case
useSingleRequestParameter: true
prefixParameterInterfaces: true
npmName: "@company/api-client"
npmVersion: "1.0.0"
```

Then generate with:

```bash
openapi-generator generate \
  -i openapi.yaml \
  -g typescript-fetch \
  -o ./generated \
  -c config.yaml
```

## Generated Structure

The generator creates the following structure:

```text
generated/
├── apis/                   # API classes
│   ├── index.ts            # Exports all APIs
│   └── *Api.ts             # Individual API classes
├── models/                 # Model/schema classes
│   ├── index.ts            # Exports all models
│   └── *.ts                # Individual model files
├── runtime.ts              # Runtime helpers (fetch, JSON conversion)
├── index.ts                # Main entry point
├── package.json            # (if withPackageJson=true)
├── tsconfig.json           # (if withPackageJson=true)
└── .openapi-generator/     # Metadata
    ├── FILES               # List of generated files
    └── VERSION             # Generator version
```

## Output Options

### File Naming

Control how generated files are named:

- **`camelCase`** (default): `petApi.ts`, `userModel.ts`
- **`kebab-case`**: `pet-api.ts`, `user-model.ts`
- **`PascalCase`**: `PetApi.ts`, `UserModel.ts`

### Enum Generation

Choose between two enum styles:

**String Enums** (`stringEnums=true`):

```typescript
export enum PetStatus {
    Available = "available",
    Pending = "pending",
    Sold = "sold"
}
```

**Const Objects** (default):

```typescript
export const PetStatus = {
    Available: "available",
    Pending: "pending",
    Sold: "sold"
} as const;

export type PetStatus = typeof PetStatus[keyof typeof PetStatus];
```

### Request Parameters

**Single Request Parameter** (`useSingleRequestParameter=true`, default):

```typescript
updatePet(requestParameters: UpdatePetRequest): Promise<Pet>
```

**Multiple Parameters** (`useSingleRequestParameter=false`):

```typescript
updatePet(petId: number, body: Pet, apiKey?: string): Promise<Pet>
```

### Runtime Checks

**With Runtime Checks** (default):

```typescript
export function PetFromJSON(json: any): Pet {
    return {
        id: json['id'],
        name: json['name'],
        status: json['status'],
    };
}
```

**Without Runtime Checks** (`withoutRuntimeChecks=true`):

```typescript
// No FromJSON/ToJSON functions generated
// Use JSON directly with type assertions
```

## Import File Extension

For ES modules compatibility, you can specify import file extensions:

```bash
openapi-generator generate \
  -i openapi.yaml \
  -g typescript-fetch \
  -o ./generated \
  -p importFileExtension=.js
```

This generates imports like:

```typescript
import { Pet } from './models/Pet.js';
```

## Custom Templates

You can customize the generated code by providing your own templates:

```bash
openapi-generator generate \
  -i openapi.yaml \
  -g typescript-fetch \
  -o ./generated \
  -t ./my-templates/typescript-fetch
```

The template directory should contain Mustache templates matching the standard
template names (see `templates/typescript-fetch/` in the repository).

## Examples

### Basic Usage

```typescript
import { Configuration, PetApi } from './generated';

const config = new Configuration({
    basePath: 'https://petstore3.swagger.io/api/v3',
});

const api = new PetApi(config);

// Fetch a pet by ID
const pet = await api.getPetById({ petId: 1 });
console.log(pet);

// Add a new pet
const newPet = await api.addPet({
    pet: {
        name: 'Fluffy',
        status: 'available',
    }
});
```

### With Authentication

```typescript
import { Configuration, PetApi } from './generated';

const config = new Configuration({
    basePath: 'https://api.example.com',
    apiKey: 'your-api-key',
    // or
    accessToken: 'your-bearer-token',
});

const api = new PetApi(config);
```

### Error Handling

```typescript
try {
    const pet = await api.getPetById({ petId: 999 });
} catch (error) {
    if (error instanceof Response) {
        console.error('HTTP Error:', error.status, await error.text());
    } else {
        console.error('Error:', error);
    }
}
```

## Compatibility

- **TypeScript**: 4.0+
- **Node.js**: 14+ (with native fetch or node-fetch)
- **Browsers**: All modern browsers with Fetch API support
- **ES Modules**: Full ESM support with `importFileExtension`

## Tips

1. **Use `withInterfaces=true`** for better flexibility when working with
   partial updates or mock data
2. **Use `stringEnums=true`** if you prefer traditional TypeScript enums
3. **Use `kebab-case`** file naming for consistency with modern frontend
   projects
4. **Enable `prefixParameterInterfaces=true`** to avoid naming conflicts in
   large APIs
5. **Use `--skip-validate-spec`** for large Swagger 2.0 specs that take too
   long to validate

## Troubleshooting

### Large Specs Take Too Long

For very large specs (like Kubernetes), skip validation:

```bash
openapi-generator generate \
  -i large-spec.json \
  -g typescript-fetch \
  -o ./generated \
  --skip-validate-spec
```

### Import Errors with ES Modules

Add the `.js` extension to imports:

```bash
-p importFileExtension=.js
```

### Type Errors with Runtime Checks

If you encounter type issues with FromJSON/ToJSON functions, consider disabling
runtime checks:

```bash
-p withoutRuntimeChecks=true
```
