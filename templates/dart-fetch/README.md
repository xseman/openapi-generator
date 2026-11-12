# dart-fetch

Generates a Dart client in the shape of typescript-fetch: an API class per
tag, three methods per operation. The output has no runtime dependencies and
needs Dart 3.13 (primary constructors).

## Usage

```bash
openapi-generator generate -g dart-fetch -i openapi.yaml -o ./petstore -p pubName=petstore
```

| Option            | Default   | Description                                                        |
| ----------------- | --------- | ------------------------------------------------------------------ |
| `pubName`         | `openapi` | name of the pub package and of its library file                    |
| `pubVersion`      | `1.0.0`   | version in `pubspec.yaml`                                          |
| `pubDescription`  |           | description in `pubspec.yaml`                                      |
| `pubHomepage`     |           | homepage in `pubspec.yaml`                                         |
| `pubRepository`   |           | repository in `pubspec.yaml`                                       |
| `sdkConstraint`   | `^3.13.0` | Dart SDK constraint in `pubspec.yaml`                              |

## Output

| File                          | Contents                                                               |
| ----------------------------- | ---------------------------------------------------------------------- |
| `lib/<pubName>.dart`          | the library: exports the runtime, the models and the APIs              |
| `lib/src/runtime.dart`        | `Configuration`, `ApiException`, `Opt`, `UploadFile`, helpers          |
| `lib/src/sender_*.dart`       | the sender of each platform: `dart:io`, `fetch` in a browser, a stub   |
| `lib/src/apis/<tag>_api.dart` | `<Tag>Api` and the enums its parameters list inline                    |
| `lib/src/models/<name>.dart`  | one model and the enums its properties list inline                     |
| `pubspec.yaml`, `README.md`   | the package and its usage                                              |

## The client

```dart
final pets = DefaultApi(Configuration(
  basePath: 'https://api.example.com',
  accessToken: () => token,
));

try {
  final pet = await pets.findPetById(id: 1);
} on ApiException catch (e) {
  print('${e.statusCode}: ${e.body}');
}
```

| Method           | Does                                                                                        |
| ---------------- | ------------------------------------------------------------------------------------------- |
| `xxx`            | sends and decodes the body: JSON, text for a `String`, a `Uint8List` for binary, a `Stream` |
| `xxxRaw`         | sends; returns the `HttpResponse` of a 2xx status                                           |
| `xxxRequestOpts` | builds the `HttpRequest` without sending it                                                 |

Parameters are named arguments, `required` where the spec says so. A result
is nullable when it is nullable in the spec or a 2xx status has no body next
to one that has. A status outside 2xx is an `ApiException` with the body as
text, and so is a 2xx status whose body is of another type than the result
(a 202 `Problem` next to a 201 `Order`).

A response of JSON Lines is a `Stream` of its items, one of server-sent
events a `Stream<ServerSentEvent<T>>`; the response stays open until the
stream is read to its end or cancelled.

## Types

| OpenAPI                              | Dart                                               |
| ------------------------------------ | -------------------------------------------------- |
| `integer`                            | `int`                                              |
| `number`                             | `double`                                           |
| `string` with `date` or `date-time`  | `DateTime`; a date is written `2024-01-31`         |
| binary body or result                | `Uint8List`                                        |
| file in a form                       | `UploadFile` (bytes and a filename)                |
| object with properties, `allOf`      | a class, the ancestors' properties included        |
| enum                                 | an `enum` with a `value`; of mixed types `Object?` |
| array, map or scalar schema          | a `typedef`                                        |
| `oneOf`, `anyOf`                     | a `sealed class`, a variant class per member       |
| `additionalProperties` by properties | a `Map` field `additionalProperties`               |

A union's variants are named after their member, `PetCat(Cat value)` or
`PetString(String value)` for a union `Pet`, so a `switch` over one is
exhaustive:

```dart
final text = switch (pet) {
  PetCat(:final value) => value.name,
  PetDog(:final value) => value.name,
  PetOther(:final value) => '$value',
};
```

`fromJson` picks the variant by the discriminator, declared or implied by a
property every member pins to a value of its own; else by the JSON type of
the value, and among objects the member it fits best: one whose required
keys it has, declaring the most of its keys. A value that is none of them is
the `Other` variant, kept as decoded.

A property that is optional and nullable is an `Opt<T>?`: null when its key
is absent, `Opt(null)` when the key holds null, `Opt(value)` otherwise, so a
PATCH can clear a field. Every other optional property is null when absent
and left out when null.

## Requests

Path, query, header, cookie and url-encoded form parameters are written in
their OpenAPI `style` and `explode` (matrix, label, spaceDelimited,
pipeDelimited, deepObject). A `multipart/form-data` body sends an
`UploadFile` as a file part, an object as a JSON part; a form with a file
goes as multipart whatever the spec declares.

A `Configuration` sends through the platform's own sender, picked by a
conditional import: one `dart:io` `HttpClient` on the VM and Flutter, whose
connections stay open between requests, `fetch` in a browser. Call
`close()` on the configuration when done, or the program waits for the
connections to idle out. `Configuration(httpSender: ...)` replaces the
transport, and `Configuration.middleware` runs `pre`, `post` and `onError`
hooks around it.

## Limits

- A union inline in a property, not a schema of its own, is an `Object?`.
- A discriminator on an `allOf` parent reads the parent only; the
  children's own properties are dropped.
- A date-time without an offset is read as local time and sent in UTC.
- An inline object is a `Map<String, Object?>`.
- Schema defaults are not constructor defaults.
