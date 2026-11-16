# tera-contrib for Go

Additional Tera filters, tests, and functions. The package uses the standard library and the implementations included in this module; there are no external Go dependencies or Cargo feature flags.

## Install

Requires Go 1.23 or later:

```sh
go get github.com/Keats/tera/tera-contrib
```

## Use

```go
package main

import (
    "fmt"
    "log"

    "github.com/Keats/tera/tera"
    contrib "github.com/Keats/tera/tera-contrib"
)

func main() {
    engine := tera.New()
    contrib.Register(engine)
    if err := engine.AddRawTemplate("post.txt", `{{ title | slug }}: {{ size | filesize_format }}`); err != nil {
        log.Fatal(err)
    }
    context := tera.NewContext()
    context.Insert("title", "Hello World!")
    context.Insert("size", 1024)
    output, err := engine.Render("post.txt", context)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(output) // hello-world: 1 KiB
}
```

Register individual extensions if you prefer different names or a smaller registry:

```go
engine.RegisterFilter("slug", contrib.Slug)
engine.RegisterFilter("regex_replace", contrib.NewRegexReplace())
engine.RegisterTest("matching", contrib.NewMatching())
engine.RegisterFunction("get_random", contrib.GetRandom)
```

Register extensions before adding templates that reference them. Filters, tests, and functions implement `tera.Filter`, `tera.Test`, and `tera.Function`, respectively.

## Extensions

| Group | Go names | Template names |
| --- | --- | --- |
| Base64 filters | `B64Encode`, `B64Decode` | `b64_encode`, `b64_decode` |
| Date filter | `Date` | `date` |
| Date tests | `IsBefore`, `IsAfter` | `before`, `after` |
| Current time function | `Now` | `now` |
| File size filter | `FilesizeFormat` | `filesize_format` |
| Formatting filter | `Format` | `format` |
| JSON filter | `JSONEncode` | `json_encode` |
| Random function | `GetRandom` | `get_random` |
| Shuffle filter | `Shuffle` | `shuffle` |
| HTML filters | `Striptags`, `Spaceless` | `striptags`, `spaceless` |
| Regular expression filter | `NewRegexReplace()` | `regex_replace` |
| Regular expression test | `NewMatching()` | `matching` |
| Slug filter | `Slug` | `slug` |
| URL encoding filters | `URLEncode`, `URLEncodeStrict` | `urlencode`, `urlencode_strict` |

`Format` uses Rust-style format specifications such as `.2`, `05`, and `>10`. `GetRandom` and `Shuffle` accept a `seed` keyword argument for reproducible results. Regular expression constructors create instances that cache compiled patterns. `Register(engine)` installs every extension using the template names in the table.

Run `go doc github.com/Keats/tera/tera-contrib` for argument details and template examples, and `go test ./tera-contrib/...` from the module root to run the tests.
