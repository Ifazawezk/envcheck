# envcheck

Checks a `.env` file against `.env.example` before you deploy. It tells you which keys are missing, which are empty, which are set twice and which values don't match the type you expect. It never prints the values themselves, so the output is safe to keep in CI logs.

One small binary, standard library only.

## Example

`.env.example` lists the keys the app needs. A comment after a key says what kind of value it takes:

```bash
# Copy this file to .env and fill in the values.
APP_ENV=development # one of: development,staging,production
PORT=8080 # int
DATABASE_URL= # url
STRIPE_SECRET_KEY=
ADMIN_EMAIL= # email
DEBUG=false # bool
SENTRY_DSN= # optional url
```

And here is a `.env` that has drifted from it:

```bash
APP_ENV=prod
PORT=8080
DATABASE_URL=postgres://app:Xk29fLq7Pw@db.internal:5432/app
STRIPE_SECRET_KEY=
DEBUG=false
REDIS_URL=redis://cache:6379
PORT=80
```

```
$ envcheck
.env:1: error: APP_ENV should be one of development, staging, production (got "****")
.env:4: error: STRIPE_SECRET_KEY is empty
.env:6: warning: REDIS_URL is not in .env.example
.env:7: error: PORT is set more than once (first on line 2)
.env.example:6: error: ADMIN_EMAIL is missing from .env
4 errors, 1 warning
```

Each problem is printed as `file:line: severity: message` on stdout, so editors and CI log viewers can jump to the line. The summary goes to stderr. `SENTRY_DSN` is not reported because it is marked optional.

## Install

```bash
go install github.com/Ifazawezk/envcheck@latest
```

## Usage

```bash
envcheck                                   # check .env against .env.example
envcheck -env .env.production              # check a different file
envcheck -env deploy/.env -example deploy/.env.example
envcheck -strict -quiet                    # extra keys fail too, print errors only
envcheck -json > envcheck.json             # machine-readable result
```

| Flag | Default | |
|---|---|---|
| `-env` | `.env` | the file to check |
| `-example` | `.env.example` | the file that lists the expected keys |
| `-strict` | off | keys that are not in the example are errors instead of warnings |
| `-json` | off | print the result as JSON |
| `-quiet` | off | print errors only: no warnings and no summary line |

Exit codes:

- `0` when there are no errors (warnings alone don't fail)
- `1` when there is at least one error
- `2` when a file can't be read or has a syntax error, or the flags are wrong

## What it checks

| Problem | Severity |
|---|---|
| key is in `.env.example` but not in `.env` | error, unless the key is marked `# optional` |
| key is in `.env` but not in `.env.example` | warning, or error with `-strict` |
| required key has an empty value | error |
| key is set more than once in either file | error |
| value doesn't match the type hint | error |

When a key is set twice, the last value is the one that gets checked, since that is what most loaders use. It is still reported as an error because not every loader agrees on which one wins.

## Type hints

A hint is a comment at the end of a line in `.env.example`. Leave a space before the `#`.

| Hint | Accepts |
|---|---|
| `# int` or `# integer` | whole numbers like `8080` or `-1` |
| `# bool` or `# boolean` | `true`, `false`, `1`, `0`, `yes`, `no`, `on`, `off`, in any case |
| `# url` | an absolute URL with a scheme and host, like `https://api.example.com` or `postgres://user:pass@db:5432/app` |
| `# email` | a plain address like `ops@example.com` |
| `# one of: debug,info,warn,error` | exactly one of the listed values (case sensitive) |
| `# optional` | the key may be missing or empty |
| `# required` | changes nothing, since keys are required unless marked optional, but it can sit next to a type: `# required int` |

`optional` can be combined with a type: `# optional url`, `# int, optional` or `# optional, one of: eu,us`. Put `optional` before `one of:`, because everything after the colon is read as the list of allowed values. Hint words can be in any case.

An empty value is never type checked. If the key is required it is reported as empty, and if it is optional it is fine.

A comment that is not made only of hint words, such as `# ask the ops team for this`, is just a comment and is ignored.

## Supported .env syntax

- `KEY=value`, with or without spaces around `=`, and an optional `export ` in front
- full-line `#` comments and blank lines
- inline comments after unquoted values, like `PORT=8080 # the port`. The `#` needs a space before it, so `COLOR=#fff` and `PASS=abc#123` keep the `#`
- single quotes, taken literally: `'no \n escapes here'`
- double quotes with `\n`, `\r`, `\t`, `\"`, `\\` and `\$` escapes
- both kinds of quotes can span several lines, which is handy for private keys and certificates
- Windows line endings and a UTF-8 byte order mark

Files must be UTF-8. UTF-16, which `>` and `>>` write in Windows PowerShell 5.1, is reported as a syntax error on the line where it starts, because most loaders can't read it either.

Keys can contain letters, digits and underscores and can't start with a digit. Variables like `${HOME}` are not expanded; the text is checked as written.

## Values are never printed

The output contains key names, file names and line numbers, never values. When a value fails a type check it is shown masked: values shorter than 8 characters become `****`, and longer ones keep only their first two characters, like `sk****`. The number of stars is always the same, so the length isn't given away either. Syntax errors don't echo the offending line, in case a secret was pasted on a line of its own.

A secret on its own line can also look like a key: `dGhpc3Rva2Vu==` is valid `KEY=value` syntax. So a name that isn't in `.env.example` is masked in the same way if it is at least 12 characters long and either mixes upper and lower case or has digits but no underscore:

```
.env:6: warning: dG**** is not in .env.example (name hidden, this line looks like a pasted secret)
```

Names like `DATABASE_URL`, `http_proxy` and `S3_BUCKET_NAME_2` are shown as usual, and so is any name the example lists. This is a check on the shape of the name, so it can't catch everything: a short token, or one that is all one case with no digits, is still printed as a name. Rotate any secret that has reached a log.

## JSON output

`-json` prints the same result as one object. This is the example above after most of the problems were fixed:

```
$ envcheck -json
{
  "env": ".env",
  "example": ".env.example",
  "ok": false,
  "errors": 1,
  "warnings": 0,
  "issues": [
    {
      "severity": "error",
      "kind": "invalid",
      "key": "APP_ENV",
      "file": ".env",
      "line": 1,
      "message": "APP_ENV should be one of development, staging, production (got \"****\")",
      "value": "****"
    }
  ]
}
```

`kind` is one of `missing`, `extra`, `empty`, `duplicate` or `invalid`. With `-quiet`, warnings are left out of `issues` but still counted in `warnings`.

## Using it in CI

In a GitHub Actions job, where the production `.env` is stored as a secret:

```yaml
- uses: actions/setup-go@v5
  with:
    go-version: stable
- run: go install github.com/Ifazawezk/envcheck@latest
- name: Check production env
  env:
    PRODUCTION_ENV: ${{ secrets.PRODUCTION_ENV }}
  run: |
    printf '%s\n' "$PRODUCTION_ENV" > .env.production
    envcheck -env .env.production -strict
```

Or at the top of a deploy script:

```bash
envcheck -strict -quiet || { echo "fix .env before deploying"; exit 1; }
```

## License

MIT, see [LICENSE](LICENSE).
