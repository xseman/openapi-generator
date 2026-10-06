# Feature specs

One small spec per generator feature, named `<area>-<feature>.yaml`: enums,
composition, discriminators, parameter styles, request bodies, responses,
security, naming edge cases. Each isolates one feature, so a change in its
generated output points at a cause.

They are the input to the regression check, which generates every spec with
the working tree and with a git ref, through every generator, once with its
defaults and once per boolean option flipped, and prints the diff:

```sh
make regress              # working tree vs HEAD
make regress REF=master   # or any branch, tag or commit
```

Run it before committing a template or parser change. No diff means the change
reached nothing else; a diff shows exactly which generated files it changed.

A new spec covers one feature and passes `openapi-generator validate -i <spec>`.
A new boolean generator option goes into `flipped` in `scripts/regress.sh`.
