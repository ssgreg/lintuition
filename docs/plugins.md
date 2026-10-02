# Plugins

Linters and classifiers both plug in, in the spirit of golangci-lint's module plugin system: they
implement the small contracts in [`sdk`](../sdk), register from `init`, and are compiled into a custom
binary. Nothing is loaded at run time.

```yaml
# .custom-lintuition.yml
version: v0.1.0            # or path: ../lintuition for a local checkout
name: custom-lintuition    # default
destination: ./bin         # default .
plugins:
  - module: example.com/lintuition-todo-owner
    version: v1.0.0
  - module: example.com/lintuition-keywords
    path: ./plugins/keywords
```

```sh
lintuition custom                 # builds ./bin/custom-lintuition
./bin/custom-lintuition version   # lists the plugins compiled in
```

Two example plugins live in their own modules under [examples/plugins](../examples/plugins): a
linter, `todo-owner`, and a local classifier, `keywords`. They import only `sdk`.

A linter is tested like an `analysistest` analyzer: [`linttest`](../linttest) runs it over examples
with and without the bug, marked with `// want` comments, and `lintuition eval` runs the same
examples against a real classifier several times with the cache off.

Plugins are trusted code: they run in-process with your rights and see every candidate, so the
payload policy binds the built-in linters, not a plugin that chooses to ignore it. The `sdk` API is
pre-v1 and may change in minor releases until v1.0.0.
